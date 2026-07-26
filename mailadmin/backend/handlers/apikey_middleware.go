package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mailadmin/models"
)

// apiKeyCtxKey namespaces the values APIKeyMiddleware injects into the request
// context so downstream handlers can scope every operation to the key's domain.
type apiKeyCtxKey string

const (
	ctxKeyDomainID   apiKeyCtxKey = "apikey_domain_id"
	ctxKeyDomainName apiKeyCtxKey = "apikey_domain_name"
)

// keyDomain returns the domain the calling API key is bound to. ok is false if
// the request did not pass through APIKeyMiddleware.
func keyDomain(r *http.Request) (id int, name string, ok bool) {
	id, ok1 := r.Context().Value(ctxKeyDomainID).(int)
	name, ok2 := r.Context().Value(ctxKeyDomainName).(string)
	return id, name, ok1 && ok2
}

// hashAPIKey returns the hex SHA-256 of a plaintext key. Only this hash is ever
// stored or compared; the plaintext exists only in transit.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// APIKeyMiddleware authenticates a request by its X-API-Key header, confirms the
// client's source address is allowlisted for that key, and scopes the request to
// the key's domain.
//
// The API is reachable two ways: directly on :8080 (real client is the TCP
// peer) and through the local nginx /api/ proxy (which sets X-Real-IP). See
// clientIP for how the true client address is derived safely in both cases.
func APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if key == "" {
			respondJSON(w, http.StatusUnauthorized, models.APIResponse{Success: false, Message: "API key required (X-API-Key header)"})
			return
		}

		var (
			id       int
			domainID int
			domain   string
			sources  string
		)
		err := db.QueryRow(
			`SELECT k.id, k.domain_id, d.name, k.allowed_sources
			 FROM domain_api_keys k
			 JOIN virtual_domains d ON d.id = k.domain_id
			 WHERE k.key_hash = ? AND k.enabled = 1`,
			hashAPIKey(key),
		).Scan(&id, &domainID, &domain, &sources)
		if errors.Is(err, sql.ErrNoRows) {
			respondJSON(w, http.StatusUnauthorized, models.APIResponse{Success: false, Message: "Invalid API key"})
			return
		}
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
			return
		}

		ip := clientIP(r)
		if ip == nil {
			respondJSON(w, http.StatusForbidden, models.APIResponse{Success: false, Message: "Could not determine client address"})
			return
		}
		if !sourceAllowed(ip, splitSources(sources)) {
			respondJSON(w, http.StatusForbidden, models.APIResponse{Success: false, Message: "Source address not allowed for this API key"})
			return
		}

		// Best-effort usage stamp; never block or fail the request on it.
		go func(keyID int) {
			db.Exec("UPDATE domain_api_keys SET last_used_at = NOW() WHERE id = ?", keyID)
		}(id)

		ctx := context.WithValue(r.Context(), ctxKeyDomainID, domainID)
		ctx = context.WithValue(ctx, ctxKeyDomainName, domain)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// clientIP derives the true client address using the trusted-proxy rule.
//
// If the TCP peer is loopback, the request came from the co-located nginx proxy
// (or a local process), so we trust the X-Real-IP / X-Forwarded-For it set.
// From any other peer we use the peer address itself and ignore forwarded
// headers, because a client hitting :8080 directly could otherwise forge them.
// An external attacker cannot make the peer appear as loopback (loopback source
// addresses are not routable from off-host), so trusting loopback is safe.
func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer != nil && peer.IsLoopback() {
		if fwd := forwardedClientIP(r); fwd != nil {
			return fwd
		}
	}
	return peer
}

// forwardedClientIP reads the client address a trusted local proxy recorded,
// preferring X-Real-IP and falling back to the first X-Forwarded-For hop.
func forwardedClientIP(r *http.Request) net.IP {
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if ip := net.ParseIP(xr); ip != nil {
			return ip
		}
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip
		}
	}
	return nil
}

// splitSources parses the stored comma/newline separated allowlist into entries.
func splitSources(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// sourceAllowed reports whether ip matches any allowlist entry. Each entry is a
// literal IP (exact match), a CIDR block (containment), or an FQDN (forward
// resolved to A/AAAA records and matched). An empty allowlist matches nothing.
func sourceAllowed(ip net.IP, entries []string) bool {
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, ipnet, err := net.ParseCIDR(entry); err == nil && ipnet.Contains(ip) {
				return true
			}
			continue
		}
		if parsed := net.ParseIP(entry); parsed != nil {
			if parsed.Equal(ip) {
				return true
			}
			continue
		}
		// Treat as FQDN and forward-resolve.
		for _, hostIP := range resolveHost(entry) {
			if hostIP.Equal(ip) {
				return true
			}
		}
	}
	return false
}

// hostResolver is overridable in tests. In production it is a plain DNS lookup.
var hostResolver = func(host string) ([]net.IP, error) { return net.LookupIP(host) }

type dnsCacheEntry struct {
	ips []net.IP
	exp time.Time
}

var (
	dnsCacheMu  sync.Mutex
	dnsCache    = map[string]dnsCacheEntry{}
	dnsCacheTTL = 60 * time.Second
)

// resolveHost forward-resolves an FQDN, caching results briefly to bound both
// latency and DNS load. Resolution failures resolve to no addresses (deny).
func resolveHost(host string) []net.IP {
	dnsCacheMu.Lock()
	if e, ok := dnsCache[host]; ok && time.Now().Before(e.exp) {
		ips := e.ips
		dnsCacheMu.Unlock()
		return ips
	}
	dnsCacheMu.Unlock()

	ips, err := hostResolver(host)
	if err != nil {
		return nil
	}

	dnsCacheMu.Lock()
	dnsCache[host] = dnsCacheEntry{ips: ips, exp: time.Now().Add(dnsCacheTTL)}
	dnsCacheMu.Unlock()
	return ips
}
