package handlers

import (
	"fmt"
	"net/http"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

// ClientSetupInfo contains email client configuration information
type ClientSetupInfo struct {
	Domain      string              `json:"domain"`
	ServerInfo  ClientServerInfo    `json:"server_info"`
	IMAP        ProtocolConfig      `json:"imap"`
	POP3        ProtocolConfig      `json:"pop3"`
	SMTP        ProtocolConfig      `json:"smtp"`
	Clients     []ClientGuide       `json:"clients"`
	Webmail     WebmailInfo         `json:"webmail"`
}

// ClientServerInfo contains server information
type ClientServerInfo struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

// ProtocolConfig contains protocol-specific settings
type ProtocolConfig struct {
	Server     string `json:"server"`
	Port       int    `json:"port"`
	PortAlt    int    `json:"port_alt,omitempty"`
	Security   string `json:"security"`
	AuthMethod string `json:"auth_method"`
}

// ClientGuide contains setup instructions for a specific client
type ClientGuide struct {
	Name         string   `json:"name"`
	Icon         string   `json:"icon"`
	Platform     string   `json:"platform"`
	Steps        []string `json:"steps"`
}

// WebmailInfo contains webmail access information
type WebmailInfo struct {
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
}

// GetClientSetup returns email client setup information for a domain
func GetClientSetup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	if domain == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Domain is required"})
		return
	}

	serverInfo := getServerInfo()
	mailServer := serverInfo.Hostname

	setup := ClientSetupInfo{
		Domain: domain,
		ServerInfo: ClientServerInfo{
			Hostname: serverInfo.Hostname,
			IP:       serverInfo.IP,
		},
		IMAP: ProtocolConfig{
			Server:     mailServer,
			Port:       993,
			Security:   "SSL/TLS",
			AuthMethod: "Normal parola",
		},
		POP3: ProtocolConfig{
			Server:     mailServer,
			Port:       995,
			Security:   "SSL/TLS",
			AuthMethod: "Normal parola",
		},
		SMTP: ProtocolConfig{
			Server:     mailServer,
			Port:       465,
			PortAlt:    587,
			Security:   "SSL/TLS (465) veya STARTTLS (587)",
			AuthMethod: "Normal parola",
		},
		Webmail: WebmailInfo{
			URL:     fmt.Sprintf("https://%s/mail", mailServer),
			Enabled: true,
		},
		Clients: []ClientGuide{
			{
				Name:     "Microsoft Outlook",
				Icon:     "outlook",
				Platform: "Windows / macOS",
				Steps: []string{
					"Dosya > Hesap Ekle'yi tıklayın",
					fmt.Sprintf("E-posta adresinizi girin (örn: kullanici@%s)", domain),
					"Gelişmiş seçenekler > Hesabımı el ile ayarlayayım'ı seçin",
					"IMAP seçeneğini tıklayın",
					fmt.Sprintf("Gelen sunucu: %s, Port: 993, SSL/TLS", mailServer),
					fmt.Sprintf("Giden sunucu: %s, Port: 465, SSL/TLS", mailServer),
					"Parolanızı girin ve Bağlan'a tıklayın",
				},
			},
			{
				Name:     "Mozilla Thunderbird",
				Icon:     "thunderbird",
				Platform: "Windows / macOS / Linux",
				Steps: []string{
					"Hesap Ayarları > Hesap Eylemleri > E-posta Hesabı Ekle",
					fmt.Sprintf("Ad, e-posta (%s) ve parolanızı girin", domain),
					"El ile yapılandır düğmesini tıklayın",
					fmt.Sprintf("IMAP: %s, Port: 993, SSL/TLS", mailServer),
					fmt.Sprintf("SMTP: %s, Port: 465, SSL/TLS", mailServer),
					"Kullanıcı adı olarak tam e-posta adresinizi girin",
					"Bitti'ye tıklayın",
				},
			},
			{
				Name:     "Apple Mail",
				Icon:     "apple",
				Platform: "macOS / iOS",
				Steps: []string{
					"Ayarlar > Mail > Hesaplar > Hesap Ekle > Diğer",
					fmt.Sprintf("E-posta: kullanici@%s", domain),
					"Parolanızı girin",
					"IMAP seçeneğini seçin",
					fmt.Sprintf("Gelen sunucu: %s", mailServer),
					fmt.Sprintf("Giden sunucu: %s", mailServer),
					"SSL kullan seçeneğini aktif edin",
					"Kaydet'e tıklayın",
				},
			},
			{
				Name:     "iPhone / iPad",
				Icon:     "ios",
				Platform: "iOS / iPadOS",
				Steps: []string{
					"Ayarlar > Mail > Hesaplar > Hesap Ekle > Diğer > Mail Hesabı Ekle",
					fmt.Sprintf("E-posta: kullanici@%s", domain),
					"Parolanızı girin, İleri'ye dokunun",
					"IMAP sekmesini seçin",
					fmt.Sprintf("Gelen posta sunucusu: %s", mailServer),
					fmt.Sprintf("Giden posta sunucusu: %s", mailServer),
					"Her iki sunucu için de kullanıcı adı olarak tam e-posta adresinizi girin",
					"Kaydet'e dokunun",
				},
			},
			{
				Name:     "Android (Gmail App)",
				Icon:     "android",
				Platform: "Android",
				Steps: []string{
					"Gmail uygulamasını açın > Ayarlar > Hesap ekle > Diğer",
					fmt.Sprintf("E-posta adresinizi girin: kullanici@%s", domain),
					"IMAP seçeneğini seçin",
					"Parolanızı girin",
					fmt.Sprintf("Gelen sunucu: %s, Port: 993, Güvenlik: SSL/TLS", mailServer),
					fmt.Sprintf("Giden sunucu: %s, Port: 465, Güvenlik: SSL/TLS", mailServer),
					"İleri'ye dokunun ve hesap ayarlarını tamamlayın",
				},
			},
			{
				Name:     "Windows Mail",
				Icon:     "windows",
				Platform: "Windows 10/11",
				Steps: []string{
					"Posta uygulamasını açın > Ayarlar > Hesapları yönet > Hesap ekle",
					"Gelişmiş kurulum > Internet e-postası seçin",
					fmt.Sprintf("E-posta: kullanici@%s", domain),
					"Kullanıcı adı: tam e-posta adresi",
					fmt.Sprintf("Gelen sunucu: %s", mailServer),
					fmt.Sprintf("Giden sunucu: %s", mailServer),
					"Hesap türü: IMAP4",
					"Tüm SSL seçeneklerini işaretleyin",
					"Oturum aç'a tıklayın",
				},
			},
		},
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: setup})
}
