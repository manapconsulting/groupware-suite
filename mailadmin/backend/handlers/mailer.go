package handlers

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"text/template"
)

// WelcomeEmailData contains data for the welcome email template
type WelcomeEmailData struct {
	Email       string
	Password    string
	Domain      string
	IMAPServer  string
	IMAPPort    int
	POP3Server  string
	POP3Port    int
	SMTPServer  string
	SMTPPort    int
	SMTPPortAlt int
	WebmailURL  string
}

const welcomeEmailTemplate = `Merhaba,

{{.Domain}} için yeni e-posta hesabınız oluşturuldu.

═══════════════════════════════════════════════════════════════════
HESAP BİLGİLERİ
═══════════════════════════════════════════════════════════════════

E-posta Adresi: {{.Email}}
Şifre: {{.Password}}

═══════════════════════════════════════════════════════════════════
SUNUCU AYARLARI
═══════════════════════════════════════════════════════════════════

GELEN POSTA (IMAP) - Önerilen
  Sunucu: {{.IMAPServer}}
  Port: {{.IMAPPort}}
  Güvenlik: SSL/TLS
  Kullanıcı adı: {{.Email}}

GELEN POSTA (POP3)
  Sunucu: {{.POP3Server}}
  Port: {{.POP3Port}}
  Güvenlik: SSL/TLS
  Kullanıcı adı: {{.Email}}

GİDEN POSTA (SMTP)
  Sunucu: {{.SMTPServer}}
  Port: {{.SMTPPort}} (SSL/TLS) veya {{.SMTPPortAlt}} (STARTTLS)
  Güvenlik: SSL/TLS veya STARTTLS
  Kullanıcı adı: {{.Email}}
  Kimlik doğrulama: Gerekli

═══════════════════════════════════════════════════════════════════
WEBMAİL ERİŞİMİ
═══════════════════════════════════════════════════════════════════

Web tarayıcısından e-postalarınıza erişmek için:
{{.WebmailURL}}

═══════════════════════════════════════════════════════════════════
E-POSTA İSTEMCİSİ KURULUMU
═══════════════════════════════════════════════════════════════════

Microsoft Outlook:
1. Dosya > Hesap Ekle'yi tıklayın
2. E-posta adresinizi girin: {{.Email}}
3. Gelişmiş seçenekler > Hesabımı el ile ayarlayayım
4. IMAP seçin
5. Gelen sunucu: {{.IMAPServer}}, Port: {{.IMAPPort}}, SSL/TLS
6. Giden sunucu: {{.SMTPServer}}, Port: {{.SMTPPort}}, SSL/TLS
7. Şifrenizi girin ve Bağlan

Mozilla Thunderbird:
1. Hesap Ayarları > Hesap Eylemleri > E-posta Hesabı Ekle
2. Ad, e-posta ve şifrenizi girin
3. El ile yapılandır düğmesini tıklayın
4. IMAP: {{.IMAPServer}}, Port: {{.IMAPPort}}, SSL/TLS
5. SMTP: {{.SMTPServer}}, Port: {{.SMTPPort}}, SSL/TLS
6. Bitti'ye tıklayın

iPhone/iPad:
1. Ayarlar > Mail > Hesaplar > Hesap Ekle > Diğer
2. E-posta ve şifrenizi girin
3. IMAP seçin
4. Gelen sunucu: {{.IMAPServer}}
5. Giden sunucu: {{.SMTPServer}}
6. Kaydet

Android (Gmail):
1. Gmail > Ayarlar > Hesap ekle > Diğer
2. E-posta adresinizi girin
3. IMAP seçin, şifrenizi girin
4. Gelen sunucu: {{.IMAPServer}}, Port: {{.IMAPPort}}, SSL/TLS
5. Giden sunucu: {{.SMTPServer}}, Port: {{.SMTPPort}}, SSL/TLS

═══════════════════════════════════════════════════════════════════
ÖNEMLİ NOTLAR
═══════════════════════════════════════════════════════════════════

• IMAP protokolü e-postalarınızı sunucuda tutar ve tüm cihazlarda
  senkronize erişim sağlar (önerilir).
• POP3 protokolü e-postaları indirir ve genellikle sunucudan siler.
• İlk girişten sonra şifrenizi değiştirmenizi öneririz.
• Şifrenizi güvenli bir yerde saklayın.

Herhangi bir sorunuz varsa sistem yöneticinizle iletişime geçin.

---
Bu e-posta otomatik olarak gönderilmiştir.
`

// SendWelcomeEmail sends account credentials and setup info to the specified email(s)
// notifyEmails can be a single email or comma-separated list of emails
func SendWelcomeEmail(notifyEmails string, data WelcomeEmailData) error {
	// Parse and clean email addresses (support comma-separated list)
	recipients := parseEmailList(notifyEmails)
	if len(recipients) == 0 {
		return fmt.Errorf("no valid email addresses provided")
	}

	// Use mail.domain as server hostname (not system hostname)
	mailServer := "mail." + data.Domain

	// Fill in server info with the user's mail domain
	data.IMAPServer = mailServer
	data.IMAPPort = 993
	data.POP3Server = mailServer
	data.POP3Port = 995
	data.SMTPServer = mailServer
	data.SMTPPort = 465
	data.SMTPPortAlt = 587
	data.WebmailURL = fmt.Sprintf("https://%s/mail", mailServer)

	// Parse and execute template
	tmpl, err := template.New("welcome").Parse(welcomeEmailTemplate)
	if err != nil {
		return fmt.Errorf("template parse error: %v", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("template execute error: %v", err)
	}

	// Build email headers
	subject := fmt.Sprintf("E-posta Hesabınız Oluşturuldu: %s", data.Email)
	from := fmt.Sprintf("noreply@%s", data.Domain)

	// Create message with proper headers
	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: Mail Admin <%s>\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body.String())

	// Send via local SMTP (localhost:25) with TLS verification disabled
	err = sendMailLocalhost(from, recipients, msg.Bytes())
	if err != nil {
		return fmt.Errorf("smtp send error: %v", err)
	}

	return nil
}

// parseEmailList parses a comma-separated list of emails and returns valid addresses
func parseEmailList(emailStr string) []string {
	var validEmails []string

	// Split by comma
	parts := strings.Split(emailStr, ",")

	for _, part := range parts {
		// Trim whitespace
		email := strings.TrimSpace(part)

		// Skip empty strings
		if email == "" {
			continue
		}

		// Basic email validation (must contain @ and have something before and after)
		if isValidEmail(email) {
			validEmails = append(validEmails, email)
		}
	}

	return validEmails
}

// isValidEmail performs basic email validation
func isValidEmail(email string) bool {
	// Must contain exactly one @
	atIndex := strings.Index(email, "@")
	if atIndex <= 0 {
		return false
	}

	// Must have something after @
	domain := email[atIndex+1:]
	if len(domain) < 3 {
		return false
	}

	// Domain must contain at least one dot
	if !strings.Contains(domain, ".") {
		return false
	}

	// No spaces allowed
	if strings.Contains(email, " ") {
		return false
	}

	return true
}

// sendMailLocalhost sends email via localhost SMTP without TLS verification
func sendMailLocalhost(from string, to []string, msg []byte) error {
	// Connect to local SMTP server
	conn, err := net.Dial("tcp", "localhost:25")
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}

	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		conn.Close()
		return fmt.Errorf("client creation failed: %v", err)
	}
	defer client.Close()

	// Try STARTTLS with InsecureSkipVerify for localhost
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         "localhost",
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			// If STARTTLS fails, continue without TLS (localhost is trusted)
			// Reconnect without TLS
			client.Close()
			conn, err = net.Dial("tcp", "localhost:25")
			if err != nil {
				return fmt.Errorf("reconnection failed: %v", err)
			}
			client, err = smtp.NewClient(conn, "localhost")
			if err != nil {
				conn.Close()
				return fmt.Errorf("client recreation failed: %v", err)
			}
			defer client.Close()
		}
	}

	// Set sender
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %v", err)
	}

	// Set recipients
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT TO failed: %v", err)
		}
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %v", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("message write failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("message close failed: %v", err)
	}

	return client.Quit()
}

// ExtractDomain extracts domain from email address
func ExtractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
