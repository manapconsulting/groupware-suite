package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type SSLStatus struct {
	Domain        string `json:"domain"`
	MailDomain    string `json:"mail_domain"`
	HasCert       bool   `json:"has_cert"`
	CertPath      string `json:"cert_path,omitempty"`
	PostfixConfig bool   `json:"postfix_configured"`
	DovecotConfig bool   `json:"dovecot_configured"`
	NginxWebmail  bool   `json:"nginx_webmail"`
	Message       string `json:"message,omitempty"`
}

type SSLConfigRequest struct {
	Domain string `json:"domain"`
}

// GetSSLStatus checks SSL certificate status for a domain
func GetSSLStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	if domain == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Domain is required",
		})
		return
	}

	mailDomain := "mail." + domain
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s", mailDomain)

	status := SSLStatus{
		Domain:     domain,
		MailDomain: mailDomain,
	}

	// Check if certificate exists
	if _, err := os.Stat(filepath.Join(certPath, "fullchain.pem")); err == nil {
		status.HasCert = true
		status.CertPath = certPath
	}

	// Check Postfix configuration
	sniMapPath := "/etc/postfix/sni_maps"
	if data, err := os.ReadFile(sniMapPath); err == nil {
		status.PostfixConfig = strings.Contains(string(data), mailDomain)
	}

	// Check Dovecot configuration
	dovecotSSLPath := "/etc/dovecot/conf.d/10-ssl.conf"
	if data, err := os.ReadFile(dovecotSSLPath); err == nil {
		status.DovecotConfig = strings.Contains(string(data), "local_name "+mailDomain)
	}

	// Check nginx webmail configuration (groupware on port 8081)
	nginxConfigPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", mailDomain)
	if data, err := os.ReadFile(nginxConfigPath); err == nil {
		status.NginxWebmail = strings.Contains(string(data), "proxy_pass http://127.0.0.1:8081")
	}

	if status.HasCert && status.PostfixConfig && status.DovecotConfig && status.NginxWebmail {
		status.Message = "SSL ve webmail tamamen yapilandirilmis"
	} else if status.HasCert && status.PostfixConfig && status.DovecotConfig {
		status.Message = "SSL tamam, webmail yapilandirilmamis"
	} else if status.HasCert {
		status.Message = "Sertifika mevcut, yapilandirma eksik"
	} else {
		status.Message = "SSL sertifikasi bulunamadi"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

// GetAllSSLStatus returns SSL status for all domains
func GetAllSSLStatus(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT name FROM virtual_domains ORDER BY name")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	defer rows.Close()

	var statuses []SSLStatus
	sniMapData, _ := os.ReadFile("/etc/postfix/sni_maps")
	dovecotData, _ := os.ReadFile("/etc/dovecot/conf.d/10-ssl.conf")

	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			continue
		}

		mailDomain := "mail." + domain
		certPath := fmt.Sprintf("/etc/letsencrypt/live/%s", mailDomain)

		status := SSLStatus{
			Domain:     domain,
			MailDomain: mailDomain,
		}

		if _, err := os.Stat(filepath.Join(certPath, "fullchain.pem")); err == nil {
			status.HasCert = true
			status.CertPath = certPath
		}

		status.PostfixConfig = strings.Contains(string(sniMapData), mailDomain)
		status.DovecotConfig = strings.Contains(string(dovecotData), "local_name "+mailDomain)

		// Check nginx webmail configuration (groupware on port 8081)
		nginxConfigPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", mailDomain)
		if data, err := os.ReadFile(nginxConfigPath); err == nil {
			status.NginxWebmail = strings.Contains(string(data), "proxy_pass http://127.0.0.1:8081")
		}

		if status.HasCert && status.PostfixConfig && status.DovecotConfig && status.NginxWebmail {
			status.Message = "Tamam"
		} else if status.HasCert && status.PostfixConfig && status.DovecotConfig {
			status.Message = "Webmail eksik"
		} else if status.HasCert {
			status.Message = "Yapilandirma eksik"
		} else {
			status.Message = "Sertifika yok"
		}

		statuses = append(statuses, status)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    statuses,
	})
}

// ConfigureSSL generates and configures SSL for a domain
func ConfigureSSL(w http.ResponseWriter, r *http.Request) {
	var req SSLConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	if req.Domain == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Domain is required",
		})
		return
	}

	mailDomain := "mail." + req.Domain
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s", mailDomain)

	// Step 1: Check if certificate already exists
	certExists := false
	if _, err := os.Stat(filepath.Join(certPath, "fullchain.pem")); err == nil {
		certExists = true
	}

	// Step 2: Generate certificate if not exists
	if !certExists {
		// First, create a temporary nginx server block for this domain
		if err := createTempNginxBlock(mailDomain); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Nginx yapilandirmasi olusturulamadi: %s", err.Error()),
			})
			return
		}

		// Reload nginx to apply the new config
		exec.Command("systemctl", "reload", "nginx").Run()

		// Run certbot with nginx plugin
		cmd := exec.Command("certbot", "--nginx",
			"-d", mailDomain,
			"--non-interactive",
			"--agree-tos",
			"--email", "admin@"+req.Domain,
			"--redirect")

		output, err := cmd.CombinedOutput()
		if err != nil {
			// Clean up temp nginx config on failure
			os.Remove(fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", mailDomain))
			os.Remove(fmt.Sprintf("/etc/nginx/sites-available/%s.conf", mailDomain))
			exec.Command("systemctl", "reload", "nginx").Run()

			respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Sertifika olusturulamadi: %s - %s", err.Error(), string(output)),
			})
			return
		}
	}

	// Step 3: Configure Postfix SNI map
	if err := configurePostfixSNI(mailDomain, certPath); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Postfix yapilandirilamadi: %s", err.Error()),
		})
		return
	}

	// Step 4: Configure Dovecot SSL
	if err := configureDovecotSSL(mailDomain, certPath); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Dovecot yapilandirilamadi: %s", err.Error()),
		})
		return
	}

	// Step 5: Reload services
	exec.Command("postmap", "-F", "/etc/postfix/sni_maps").Run()
	exec.Command("systemctl", "reload", "postfix").Run()
	exec.Command("systemctl", "reload", "dovecot").Run()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("%s icin SSL basariyla yapilandirildi", mailDomain),
	})
}

// ConfigureNginxWebmail configures nginx with SOGo proxy for a domain
func ConfigureNginxWebmail(w http.ResponseWriter, r *http.Request) {
	var req SSLConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	if req.Domain == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Domain is required",
		})
		return
	}

	mailDomain := "mail." + req.Domain
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s", mailDomain)

	// Check if certificate exists
	if _, err := os.Stat(filepath.Join(certPath, "fullchain.pem")); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "SSL sertifikasi bulunamadi. Oncelikle SSL yapilandirin.",
		})
		return
	}

	// Create nginx config with SOGo proxy
	if err := createNginxWebmailConfig(mailDomain, certPath); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Nginx yapilandirilamadi: %s", err.Error()),
		})
		return
	}

	// Test nginx configuration
	cmd := exec.Command("nginx", "-t")
	if output, err := cmd.CombinedOutput(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Nginx yapilandirma hatasi: %s", string(output)),
		})
		return
	}

	// Reload nginx
	exec.Command("systemctl", "reload", "nginx").Run()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("%s icin webmail yapilandirildi", mailDomain),
	})
}

func createNginxWebmailConfig(mailDomain, certPath string) error {
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", mailDomain)
	enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", mailDomain)

	// Extract base domain for logging
	baseDomain := strings.TrimPrefix(mailDomain, "mail.")

	config := fmt.Sprintf(`# Groupware - %s
server {
    listen 80;
    listen [::]:80;
    server_name %s;

    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;

    # SSL Certificates
    ssl_certificate %s/fullchain.pem;
    ssl_certificate_key %s/privkey.pem;

    # SSL Configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Logging
    access_log /var/log/nginx/groupware-%s-access.log;
    error_log /var/log/nginx/groupware-%s-error.log;

    # Max upload size
    client_max_body_size 50M;

    # Redirect root to /mail/
    location = / {
        return 302 /mail/;
    }

    # Proxy to Groupware application (port 8081)
    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_redirect off;

        # Headers
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts
        proxy_connect_timeout 90;
        proxy_send_timeout 90;
        proxy_read_timeout 90;

        # Buffer sizes
        proxy_buffer_size 128k;
        proxy_buffers 8 128k;
        proxy_busy_buffers_size 256k;
    }

    # CalDAV/CardDAV well-known redirects
    location /.well-known/caldav {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /.well-known/carddav {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`, baseDomain, mailDomain, mailDomain, certPath, certPath, baseDomain, baseDomain)

	// Write config file
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return err
	}

	// Remove existing symlink if it exists
	os.Remove(enabledPath)

	// Create symlink to enable the site
	return os.Symlink(configPath, enabledPath)
}

func configurePostfixSNI(mailDomain, certPath string) error {
	sniMapPath := "/etc/postfix/sni_maps"

	// Read existing content
	data, err := os.ReadFile(sniMapPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if already configured
	if strings.Contains(string(data), mailDomain) {
		return nil // Already configured
	}

	// Append new entry
	entry := fmt.Sprintf("%s %s/privkey.pem %s/fullchain.pem\n",
		mailDomain, certPath, certPath)

	f, err := os.OpenFile(sniMapPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}

func createTempNginxBlock(mailDomain string) error {
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", mailDomain)
	enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s.conf", mailDomain)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // Already exists
	}

	// Create a minimal server block for SSL certificate generation
	config := fmt.Sprintf(`server {
    listen 80;
    server_name %s;

    location / {
        return 200 'Mail server: %s';
        add_header Content-Type text/plain;
    }
}
`, mailDomain, mailDomain)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return err
	}

	// Create symlink to enable the site
	return os.Symlink(configPath, enabledPath)
}

func configureDovecotSSL(mailDomain, certPath string) error {
	dovecotSSLPath := "/etc/dovecot/conf.d/10-ssl.conf"

	// Read existing content
	data, err := os.ReadFile(dovecotSSLPath)
	if err != nil {
		return err
	}

	content := string(data)

	// Check if already configured
	if strings.Contains(content, "local_name "+mailDomain) {
		return nil // Already configured
	}

	// Find the position to insert (before ssl_key_password or at end)
	insertConfig := fmt.Sprintf(`
local_name %s {
  ssl_cert = <%s/fullchain.pem
  ssl_key = <%s/privkey.pem
}
`, mailDomain, certPath, certPath)

	// Insert before #ssl_key_password or append
	insertPos := strings.Index(content, "#ssl_key_password")
	if insertPos == -1 {
		insertPos = strings.Index(content, "# root owned")
	}

	if insertPos > 0 {
		content = content[:insertPos] + insertConfig + content[insertPos:]
	} else {
		content += insertConfig
	}

	return os.WriteFile(dovecotSSLPath, []byte(content), 0644)
}
