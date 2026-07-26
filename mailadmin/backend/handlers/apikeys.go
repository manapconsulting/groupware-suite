package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

// apiKeyLabel prefixes every generated key so it is recognisable in logs and
// config files ("vendisens mail key" ~ vmk).
const apiKeyLabel = "vmk_"

// generateAPIKey returns a new random key plus the short prefix stored for
// display. The full key is returned to the caller once and never persisted.
func generateAPIKey() (key, prefix string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	key = apiKeyLabel + base64.RawURLEncoding.EncodeToString(buf)
	prefix = key
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return key, prefix, nil
}

// validateSources normalises and checks an allowlist: every entry must be a
// literal IP, a CIDR block, or an FQDN. A key with no sources would be unusable
// (nothing matches), so an empty list is rejected.
func validateSources(entries []string) ([]string, error) {
	cleaned := make([]string, 0, len(entries))
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		switch {
		case strings.Contains(e, "/"):
			if _, _, err := net.ParseCIDR(e); err != nil {
				return nil, fmt.Errorf("invalid CIDR %q", e)
			}
		case net.ParseIP(e) != nil:
			// literal IP, ok
		case domainRE.MatchString(e) && !allNumericTLD(e):
			// FQDN, ok
		default:
			return nil, fmt.Errorf("invalid source %q (must be an IP, CIDR or FQDN)", e)
		}
		cleaned = append(cleaned, e)
	}
	if len(cleaned) == 0 {
		return nil, errors.New("at least one allowed source (IP, CIDR or FQDN) is required")
	}
	return cleaned, nil
}

// allNumericTLD reports whether a host's last label is all digits. Real FQDNs
// never end in an all-numeric label (RFC 3696), so this catches malformed
// dotted-decimal IPs (e.g. "999.999.999.999") that would otherwise slip through
// the FQDN check.
func allNumericTLD(host string) bool {
	labels := strings.Split(host, ".")
	tld := labels[len(labels)-1]
	for _, c := range tld {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// domainExists reports whether a domain id is present and returns its name.
func domainName(id int) (string, bool) {
	var name string
	err := db.QueryRow("SELECT name FROM virtual_domains WHERE id = ?", id).Scan(&name)
	if err != nil {
		return "", false
	}
	return name, true
}

// GetDomainAPIKeys lists the keys for a domain. Hashes and plaintext are never
// returned -- only the display prefix and metadata.
func GetDomainAPIKeys(w http.ResponseWriter, r *http.Request) {
	domainID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid domain ID"})
		return
	}

	rows, err := db.Query(
		`SELECT id, domain_id, name, key_prefix, allowed_sources, enabled,
		        COALESCE(last_used_at, ''), COALESCE(created_at, '')
		 FROM domain_api_keys WHERE domain_id = ? ORDER BY created_at DESC`,
		domainID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer rows.Close()

	keys := []models.DomainAPIKey{}
	for rows.Next() {
		var (
			k        models.DomainAPIKey
			sources  string
			enabled  int
			lastUsed string
			created  string
		)
		if err := rows.Scan(&k.ID, &k.DomainID, &k.Name, &k.KeyPrefix, &sources, &enabled, &lastUsed, &created); err != nil {
			continue
		}
		k.AllowedSources = splitSources(sources)
		k.Enabled = enabled == 1
		k.LastUsedAt = lastUsed
		k.CreatedAt = created
		keys = append(keys, k)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: keys})
}

// CreateDomainAPIKey mints a key for a domain and returns the plaintext exactly
// once.
func CreateDomainAPIKey(w http.ResponseWriter, r *http.Request) {
	domainID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid domain ID"})
		return
	}
	name, ok := domainName(domainID)
	if !ok {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Unknown domain"})
		return
	}

	var req models.APIKeyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Key name is required"})
		return
	}
	sources, err := validateSources(req.AllowedSources)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	enabled := 1
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}

	key, prefix, err := generateAPIKey()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Failed to generate key: " + err.Error()})
		return
	}

	result, err := db.Exec(
		`INSERT INTO domain_api_keys (domain_id, name, key_prefix, key_hash, allowed_sources, enabled)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		domainID, req.Name, prefix, hashAPIKey(key), strings.Join(sources, ","), enabled,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	id, _ := result.LastInsertId()

	resp := models.APIKeyCreateResponse{
		DomainAPIKey: models.DomainAPIKey{
			ID:             int(id),
			DomainID:       domainID,
			Domain:         name,
			Name:           req.Name,
			KeyPrefix:      prefix,
			AllowedSources: sources,
			Enabled:        enabled == 1,
		},
		Key: key,
	}
	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: resp})
}

// UpdateDomainAPIKey changes a key's name, allowed sources, and enabled flag.
// The key material itself is immutable -- rotate by deleting and recreating.
func UpdateDomainAPIKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid domain ID"})
		return
	}
	keyID, err := strconv.Atoi(vars["keyId"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid key ID"})
		return
	}

	var req models.APIKeyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Key name is required"})
		return
	}
	sources, err := validateSources(req.AllowedSources)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	enabled := 1
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}

	// Scope the update to the domain in the URL so one domain can't touch
	// another's keys.
	result, err := db.Exec(
		`UPDATE domain_api_keys SET name = ?, allowed_sources = ?, enabled = ?
		 WHERE id = ? AND domain_id = ?`,
		req.Name, strings.Join(sources, ","), enabled, keyID, domainID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "API key not found"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "API key updated"})
}

// DeleteDomainAPIKey revokes a key immediately.
func DeleteDomainAPIKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid domain ID"})
		return
	}
	keyID, err := strconv.Atoi(vars["keyId"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid key ID"})
		return
	}

	result, err := db.Exec("DELETE FROM domain_api_keys WHERE id = ? AND domain_id = ?", keyID, domainID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "API key not found"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "API key revoked"})
}
