package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

// Custom DNS resolver using Google DNS (8.8.8.8) for fresh lookups
var dnsResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: time.Second * 10,
		}
		return d.DialContext(ctx, "udp", "8.8.8.8:53")
	},
}

// DNSCheckResult represents the result of a DNS check
type DNSCheckResult struct {
	Check     string `json:"check"`
	Status    string `json:"status"` // ok, warning, error
	Message   string `json:"message"`
	Value     string `json:"value,omitempty"`
	Help      string `json:"help,omitempty"`
	Suggested string `json:"suggested,omitempty"` // Ready-to-use DNS record
}

// DomainCheckResponse is the full response for domain checks
type DomainCheckResponse struct {
	Domain   string           `json:"domain"`
	Checks   []DNSCheckResult `json:"checks"`
	Overall  string           `json:"overall"` // ok, warning, error
	ServerIP string           `json:"server_ip"`
	Hostname string           `json:"hostname"`
}

// ServerInfo holds the server's network information
type ServerInfo struct {
	IP       string
	Hostname string
}

// getServerInfo returns the mail server's IP and hostname
func getServerInfo() ServerInfo {
	info := ServerInfo{
		IP:       "127.0.0.1",
		Hostname: "localhost",
	}

	// Get public IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		info.IP = localAddr.IP.String()
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err == nil {
		info.Hostname = hostname
	}

	// Try to get FQDN from reverse DNS
	names, err := net.LookupAddr(info.IP)
	if err == nil && len(names) > 0 {
		// Remove trailing dot
		fqdn := strings.TrimSuffix(names[0], ".")
		if fqdn != "" {
			info.Hostname = fqdn
		}
	}

	return info
}

// CheckDomainDNS performs all DNS checks for a domain
func CheckDomainDNS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	if domain == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Domain is required"})
		return
	}

	serverInfo := getServerInfo()
	var checks []DNSCheckResult
	overall := "ok"

	// 1. MX Record Check
	mxCheck := checkMX(domain, serverInfo)
	checks = append(checks, mxCheck)
	if mxCheck.Status == "error" {
		overall = "error"
	}

	// 2. SPF Record Check
	spfCheck := checkSPF(domain, serverInfo)
	checks = append(checks, spfCheck)
	if spfCheck.Status == "error" && overall != "error" {
		overall = "warning"
	}
	if spfCheck.Status == "warning" && overall == "ok" {
		overall = "warning"
	}

	// 3. DKIM Record Check
	dkimCheck := checkDKIM(domain, serverInfo)
	checks = append(checks, dkimCheck)
	if dkimCheck.Status == "error" && overall != "error" {
		overall = "warning"
	}
	if dkimCheck.Status == "warning" && overall == "ok" {
		overall = "warning"
	}

	// 4. DMARC Record Check
	dmarcCheck := checkDMARC(domain, serverInfo)
	checks = append(checks, dmarcCheck)
	if dmarcCheck.Status == "error" && overall != "error" {
		overall = "warning"
	}
	if dmarcCheck.Status == "warning" && overall == "ok" {
		overall = "warning"
	}

	// 5. PTR Record Check (reverse DNS)
	ptrCheck := checkPTR(serverInfo)
	checks = append(checks, ptrCheck)
	if ptrCheck.Status == "warning" && overall == "ok" {
		overall = "warning"
	}

	// 6. Blacklist Check
	blCheck := checkBlacklist(serverInfo)
	checks = append(checks, blCheck)
	if blCheck.Status == "error" {
		overall = "error"
	}

	response := DomainCheckResponse{
		Domain:   domain,
		Checks:   checks,
		Overall:  overall,
		ServerIP: serverInfo.IP,
		Hostname: serverInfo.Hostname,
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: response})
}

func checkMX(domain string, server ServerInfo) DNSCheckResult {
	ctx := context.Background()
	mxRecords, err := dnsResolver.LookupMX(ctx, domain)
	if err != nil || len(mxRecords) == 0 {
		// Suggest mail.domain.com as MX
		suggestedMX := fmt.Sprintf("mail.%s", domain)
		return DNSCheckResult{
			Check:     "MX",
			Status:    "error",
			Message:   "MX kaydı bulunamadı - E-posta alamazsınız!",
			Help:      "Domain sağlayıcınızın DNS panelinden MX kaydı ekleyin.",
			Suggested: fmt.Sprintf("%s.  IN  MX  10  %s.", domain, suggestedMX),
		}
	}

	var mxHosts []string
	for _, mx := range mxRecords {
		mxHosts = append(mxHosts, fmt.Sprintf("%s (öncelik: %d)", strings.TrimSuffix(mx.Host, "."), mx.Pref))
	}

	return DNSCheckResult{
		Check:   "MX",
		Status:  "ok",
		Message: "MX kaydı doğru yapılandırılmış",
		Value:   strings.Join(mxHosts, ", "),
	}
}

func checkSPF(domain string, server ServerInfo) DNSCheckResult {
	ctx := context.Background()
	txtRecords, err := dnsResolver.LookupTXT(ctx, domain)
	if err != nil {
		// Suggest a secure SPF record
		suggested := fmt.Sprintf("v=spf1 mx ip4:%s -all", server.IP)
		return DNSCheckResult{
			Check:     "SPF",
			Status:    "error",
			Message:   "SPF kaydı okunamadı",
			Help:      "DNS sunucusunu kontrol edin veya SPF kaydı ekleyin.",
			Suggested: fmt.Sprintf("%s.  IN  TXT  \"%s\"", domain, suggested),
		}
	}

	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "v=spf1") {
			// Check security level
			hasHardFail := strings.Contains(txt, "-all")
			hasSoftFail := strings.Contains(txt, "~all")
			hasNeutral := strings.Contains(txt, "?all")

			// Check if our server is included
			serverIncluded := strings.Contains(txt, "mx") ||
				strings.Contains(txt, fmt.Sprintf("ip4:%s", server.IP)) ||
				strings.Contains(txt, "a:")

			if !serverIncluded {
				suggested := fmt.Sprintf("v=spf1 mx ip4:%s -all", server.IP)
				return DNSCheckResult{
					Check:     "SPF",
					Status:    "warning",
					Message:   "SPF kaydı var ama bu sunucu dahil değil",
					Value:     txt,
					Help:      fmt.Sprintf("SPF kaydına sunucu IP'sini ekleyin: ip4:%s", server.IP),
					Suggested: fmt.Sprintf("%s.  IN  TXT  \"%s\"", domain, suggested),
				}
			}

			if hasNeutral {
				suggested := strings.Replace(txt, "?all", "-all", 1)
				return DNSCheckResult{
					Check:     "SPF",
					Status:    "warning",
					Message:   "SPF kaydı var ama '?all' güvenli değil",
					Value:     txt,
					Help:      "Güvenlik için '?all' yerine '-all' kullanın (kesin reddet).",
					Suggested: fmt.Sprintf("%s.  IN  TXT  \"%s\"", domain, suggested),
				}
			}

			if hasSoftFail && !hasHardFail {
				return DNSCheckResult{
					Check:   "SPF",
					Status:  "ok",
					Message: "SPF kaydı doğru (~all: soft fail)",
					Value:   txt,
					Help:    "İpucu: Daha güvenli için '~all' yerine '-all' kullanabilirsiniz.",
				}
			}

			return DNSCheckResult{
				Check:   "SPF",
				Status:  "ok",
				Message: "SPF kaydı güvenli şekilde yapılandırılmış",
				Value:   txt,
			}
		}
	}

	// No SPF record found - suggest a secure one
	suggested := fmt.Sprintf("v=spf1 mx ip4:%s -all", server.IP)
	return DNSCheckResult{
		Check:     "SPF",
		Status:    "error",
		Message:   "SPF kaydı bulunamadı - Spoofing riski!",
		Help:      "SPF kaydı olmadan başkaları sizin adınıza sahte e-posta gönderebilir.",
		Suggested: fmt.Sprintf("%s.  IN  TXT  \"%s\"", domain, suggested),
	}
}

func checkDKIM(domain string, server ServerInfo) DNSCheckResult {
	ctx := context.Background()
	// Check for common DKIM selectors
	selectors := []string{"default", "mail", "dkim", "selector1", "selector2"}

	for _, selector := range selectors {
		dkimDomain := fmt.Sprintf("%s._domainkey.%s", selector, domain)
		txtRecords, err := dnsResolver.LookupTXT(ctx, dkimDomain)
		if err == nil && len(txtRecords) > 0 {
			for _, txt := range txtRecords {
				if strings.Contains(txt, "v=DKIM1") || strings.Contains(txt, "p=") {
					// Check key length hint
					keyPart := ""
					if idx := strings.Index(txt, "p="); idx != -1 {
						keyPart = txt[idx+2:]
						if semicolon := strings.Index(keyPart, ";"); semicolon != -1 {
							keyPart = keyPart[:semicolon]
						}
					}

					displayTxt := txt
					if len(txt) > 80 {
						displayTxt = txt[:80] + "..."
					}

					return DNSCheckResult{
						Check:   "DKIM",
						Status:  "ok",
						Message: fmt.Sprintf("DKIM kaydı bulundu (selector: %s)", selector),
						Value:   displayTxt,
					}
				}
			}
		}
	}

	// Try to read DKIM key from server
	dkimKey := getDKIMKeyContent(domain)
	help := "OpenDKIM yapılandırmasını kontrol edin: sudo opendkim-genkey -s default -d " + domain

	if dkimKey != "" {
		return DNSCheckResult{
			Check:     "DKIM",
			Status:    "warning",
			Message:   "DKIM anahtarı sunucuda var ama DNS'te yok",
			Help:      "Aşağıdaki kaydı DNS'e ekleyin:",
			Suggested: dkimKey,
		}
	}

	return DNSCheckResult{
		Check:     "DKIM",
		Status:    "warning",
		Message:   "DKIM kaydı bulunamadı",
		Help:      help,
		Suggested: fmt.Sprintf("default._domainkey.%s.  IN  TXT  \"v=DKIM1; k=rsa; p=<PUBLIC_KEY>\"", domain),
	}
}

func checkDMARC(domain string, server ServerInfo) DNSCheckResult {
	ctx := context.Background()
	dmarcDomain := fmt.Sprintf("_dmarc.%s", domain)
	txtRecords, err := dnsResolver.LookupTXT(ctx, dmarcDomain)

	// Secure DMARC suggestion with reporting
	suggested := fmt.Sprintf("v=DMARC1; p=quarantine; sp=quarantine; rua=mailto:dmarc@%s; ruf=mailto:dmarc@%s; fo=1; adkim=s; aspf=s; pct=100", domain, domain)

	if err != nil {
		return DNSCheckResult{
			Check:     "DMARC",
			Status:    "warning",
			Message:   "DMARC kaydı bulunamadı",
			Help:      "DMARC, SPF ve DKIM'i birleştiren bir güvenlik politikasıdır.",
			Suggested: fmt.Sprintf("_dmarc.%s.  IN  TXT  \"%s\"", domain, suggested),
		}
	}

	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "v=DMARC1") {
			// Check policy strength
			hasReject := strings.Contains(txt, "p=reject")
			hasQuarantine := strings.Contains(txt, "p=quarantine")
			hasNone := strings.Contains(txt, "p=none")
			hasReporting := strings.Contains(txt, "rua=")

			if hasNone {
				stricterSuggested := strings.Replace(txt, "p=none", "p=quarantine", 1)
				return DNSCheckResult{
					Check:     "DMARC",
					Status:    "warning",
					Message:   "DMARC politikası 'none' - Koruma sağlamıyor!",
					Value:     txt,
					Help:      "Test aşamasını geçtiyseniz 'p=quarantine' veya 'p=reject' kullanın.",
					Suggested: fmt.Sprintf("_dmarc.%s.  IN  TXT  \"%s\"", domain, stricterSuggested),
				}
			}

			if !hasReporting {
				return DNSCheckResult{
					Check:   "DMARC",
					Status:  "ok",
					Message: "DMARC kaydı var (raporlama önerilir)",
					Value:   txt,
					Help:    "İpucu: rua= ekleyerek DMARC raporları alabilirsiniz.",
				}
			}

			status := "ok"
			message := "DMARC kaydı güvenli şekilde yapılandırılmış"
			if hasQuarantine {
				message = "DMARC kaydı iyi yapılandırılmış (quarantine)"
			}
			if hasReject {
				message = "DMARC kaydı en güvenli şekilde yapılandırılmış (reject)"
			}

			return DNSCheckResult{
				Check:   "DMARC",
				Status:  status,
				Message: message,
				Value:   txt,
			}
		}
	}

	return DNSCheckResult{
		Check:     "DMARC",
		Status:    "warning",
		Message:   "DMARC kaydı bulunamadı",
		Help:      "DMARC, SPF ve DKIM'i birleştiren bir güvenlik politikasıdır.",
		Suggested: fmt.Sprintf("_dmarc.%s.  IN  TXT  \"%s\"", domain, suggested),
	}
}

func checkPTR(server ServerInfo) DNSCheckResult {
	names, err := net.LookupAddr(server.IP)
	if err != nil || len(names) == 0 {
		return DNSCheckResult{
			Check:     "PTR",
			Status:    "warning",
			Message:   fmt.Sprintf("Reverse DNS (PTR) kaydı yok (%s)", server.IP),
			Help:      "PTR kaydı olmadan bazı mail sunucuları e-postalarınızı reddedebilir.",
			Suggested: fmt.Sprintf("Hosting sağlayıcınızdan %s için PTR kaydı isteyin: %s", server.IP, server.Hostname),
		}
	}

	ptrName := strings.TrimSuffix(names[0], ".")
	return DNSCheckResult{
		Check:   "PTR",
		Status:  "ok",
		Message: "Reverse DNS doğru yapılandırılmış",
		Value:   fmt.Sprintf("%s → %s", server.IP, ptrName),
	}
}

// BlacklistInfo contains information about a blacklist
type BlacklistInfo struct {
	Name        string
	DNSBL       string
	LookupURL   string
	RemovalURL  string
	Description string
}

var blacklistInfos = []BlacklistInfo{
	{
		Name:        "Spamhaus ZEN",
		DNSBL:       "zen.spamhaus.org",
		LookupURL:   "https://check.spamhaus.org/listed/?searchterm=%s",
		RemovalURL:  "https://check.spamhaus.org/listed/?searchterm=%s",
		Description: "En yaygın kullanılan blacklist. SBL, XBL, PBL listelerini içerir.",
	},
	{
		Name:        "SpamCop",
		DNSBL:       "bl.spamcop.net",
		LookupURL:   "https://www.spamcop.net/bl.shtml?%s",
		RemovalURL:  "https://www.spamcop.net/bl.shtml?%s",
		Description: "Spam şikayetlerine dayalı liste. 24-48 saat içinde otomatik düşer.",
	},
	{
		Name:        "Barracuda",
		DNSBL:       "b.barracudacentral.org",
		LookupURL:   "https://www.barracudacentral.org/lookups/lookup-reputation?lookup_entry=%s",
		RemovalURL:  "https://www.barracudacentral.org/lookups/lookup-reputation?lookup_entry=%s",
		Description: "Barracuda güvenlik ürünleri tarafından kullanılır.",
	},
	{
		Name:        "SORBS",
		DNSBL:       "dnsbl.sorbs.net",
		LookupURL:   "http://www.sorbs.net/lookup.shtml?%s",
		RemovalURL:  "http://www.sorbs.net/cgi-bin/support?%s",
		Description: "Spam ve açık relay sunucuları listeler.",
	},
	{
		Name:        "UCEPROTECT Level 1",
		DNSBL:       "dnsbl-1.uceprotect.net",
		LookupURL:   "https://www.uceprotect.net/en/rblcheck.php?ipr=%s",
		RemovalURL:  "https://www.uceprotect.net/en/rblcheck.php?ipr=%s",
		Description: "Tek IP bazlı listeleme. 7 gün sonra otomatik düşer.",
	},
	{
		Name:        "Spamrats",
		DNSBL:       "dyna.spamrats.com",
		LookupURL:   "https://www.spamrats.com/lookup.php?ip=%s",
		RemovalURL:  "https://www.spamrats.com/lookup.php?ip=%s",
		Description: "Dinamik IP ve spam kaynakları listeler.",
	},
}

func checkBlacklist(server ServerInfo) DNSCheckResult {
	// Reverse the IP for DNSBL lookup
	parts := strings.Split(server.IP, ".")
	if len(parts) != 4 {
		return DNSCheckResult{
			Check:   "Blacklist",
			Status:  "warning",
			Message: "IP formatı kontrol edilemedi",
		}
	}
	reversedIP := fmt.Sprintf("%s.%s.%s.%s", parts[3], parts[2], parts[1], parts[0])

	ctx := context.Background()
	var listedOn []BlacklistInfo
	var checkedCount int

	for _, bl := range blacklistInfos {
		query := fmt.Sprintf("%s.%s", reversedIP, bl.DNSBL)
		_, err := dnsResolver.LookupHost(ctx, query)
		if err == nil {
			// If lookup succeeds, IP is on blacklist
			listedOn = append(listedOn, bl)
		}
		checkedCount++
	}

	if len(listedOn) > 0 {
		// Build detailed help message with removal links
		var details []string
		for _, bl := range listedOn {
			removalURL := fmt.Sprintf(bl.RemovalURL, server.IP)
			details = append(details, fmt.Sprintf("• %s: %s\n  Çıkış: %s", bl.Name, bl.Description, removalURL))
		}

		var names []string
		for _, bl := range listedOn {
			names = append(names, bl.Name)
		}

		return DNSCheckResult{
			Check:   "Blacklist",
			Status:  "error",
			Message: fmt.Sprintf("Sunucu IP'si %d blacklist'te bulundu!", len(listedOn)),
			Value:   strings.Join(names, ", "),
			Help:    fmt.Sprintf("IP: %s\n\nListeden çıkış için aşağıdaki linkleri kullanın:\n\n%s", server.IP, strings.Join(details, "\n\n")),
		}
	}

	return DNSCheckResult{
		Check:   "Blacklist",
		Status:  "ok",
		Message: "Sunucu hiçbir blacklist'te değil",
		Value:   fmt.Sprintf("Kontrol edilen: %d liste (%s)", checkedCount, server.IP),
	}
}

// getDKIMKeyContent tries to read DKIM public key from server
func getDKIMKeyContent(domain string) string {
	keyPaths := []string{
		fmt.Sprintf("/etc/opendkim/keys/%s/default.txt", domain),
		fmt.Sprintf("/etc/opendkim/keys/%s/mail.txt", domain),
		"/etc/opendkim/keys/default.txt",
	}

	for _, path := range keyPaths {
		content, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return ""
}

// GetDKIMKey returns the DKIM public key for a domain
func GetDKIMKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	keyContent := getDKIMKeyContent(domain)
	if keyContent != "" {
		respondJSON(w, http.StatusOK, models.APIResponse{
			Success: true,
			Data: map[string]string{
				"key":      keyContent,
				"selector": "default",
				"domain":   domain,
			},
		})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: false,
		Message: fmt.Sprintf("DKIM key dosyası bulunamadı. Oluşturmak için: sudo opendkim-genkey -s default -d %s -D /etc/opendkim/keys/%s", domain, domain),
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
