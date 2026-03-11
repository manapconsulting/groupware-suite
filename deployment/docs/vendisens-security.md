# vendisens.com — E-Posta Güvenlik Önlemleri

**Hazırlayan:** Manap Consulting
**Tarih:** 11 Mart 2026
**Sunucu:** 167.86.70.187 — mail.vendisens.com
**Kapsam:** Postfix + Dovecot + ClamAV + Amavis + DNS güvenlik katmanları

---

## İçindekiler

1. [Mimari Genel Bakış](#1-mimari-genel-bakış)
2. [DNS Güvenlik Katmanları](#2-dns-güvenlik-katmanları)
3. [TLS / Şifreleme](#3-tls--şifreleme)
4. [Kimlik Doğrulama](#4-kimlik-doğrulama)
5. [Gelen Mail Filtreleme](#5-gelen-mail-filtreleme)
6. [Virüs ve Spam Tarama](#6-virüs-ve-spam-tarama)
7. [Brute-Force ve DDoS Koruması](#7-brute-force-ve-ddos-koruması)
8. [Güvenlik Duvarı](#8-güvenlik-duvarı)
9. [SSL Sertifika Yönetimi](#9-ssl-sertifika-yönetimi)
10. [Dosya Sistemi Güvenliği](#10-dosya-sistemi-güvenliği)
11. [İzleme ve Raporlama](#11-i̇zleme-ve-raporlama)
12. [Bakım Prosedürleri](#12-bakım-prosedürleri)
13. [Konfigürasyon Referansı](#13-konfigürasyon-referansı)

---

## 1. Mimari Genel Bakış

```
İnternet
    │
    ▼
┌──────────────────────────────────────────────┐
│  UFW Güvenlik Duvarı                         │
│  Açık portlar: 22,25,80,143,443,465,587,993  │
└──────────────────────────────────────────────┘
    │
    ▼
┌──────────────────────────────────────────────┐
│  Cloudflare DNS + Proxy (vendisens.com)      │
│  SPF / DKIM / DMARC doğrulaması (gönderen)  │
│  MTA-STS (zorunlu TLS aktarımı)              │
└──────────────────────────────────────────────┘
    │
    ▼ Port 25
┌──────────────────────────────────────────────┐
│  POSTSCREEN (Bot Filtresi)                   │
│  • DNSBL kontrolü (Spamhaus, SpamCop,        │
│    Barracuda)                                │
│  • Greeting banner testi (enforce)           │
│  • Kara liste: bağlantı kesilir (drop)       │
└──────────────────────────────────────────────┘
    │ Temiz bağlantılar
    ▼
┌──────────────────────────────────────────────┐
│  POSTFIX smtpd                               │
│  • TLS 1.2+ zorunlu                          │
│  • HELO/EHLO doğrulaması                     │
│  • Gönderen kimlik kontrolü (SPF maps)       │
│  • Alıcı kısıtlamaları                       │
│  • Rate limiting                             │
└──────────────────────────────────────────────┘
    │ content_filter
    ▼ Port 10024
┌──────────────────────────────────────────────┐
│  AMAVIS                                      │
│  • ClamAV ile virüs tarama                   │
│  • SpamAssassin ile spam skoru               │
│  • Yasaklı ek kontrolü                       │
└──────────────────────────────────────────────┘
    │ Temiz mail
    ▼ Port 10025 → Postfix → LMTP
┌──────────────────────────────────────────────┐
│  DOVECOT                                     │
│  • IMAP/IMAPS (143/993)                      │
│  • TLS 1.2+, ECDHE cipher suite              │
│  • SASL kimlik doğrulama                     │
└──────────────────────────────────────────────┘
    │
    ▼
┌──────────────────────────────────────────────┐
│  MySQL — mailserver DB                       │
│  virtual_domains / virtual_users /           │
│  virtual_aliases                             │
└──────────────────────────────────────────────┘
```

---

## 2. DNS Güvenlik Katmanları

### 2.1 SPF (Sender Policy Framework)

**DNS kaydı:**
```
vendisens.com.  TXT  "v=spf1 mx a:mail.vendisens.com ip4:167.86.70.187 -all"
```

**Nasıl çalışır:**
- Yalnızca `167.86.70.187` ve `mail.vendisens.com`'dan gönderim yetkisi tanınır
- `-all` → diğer tüm kaynaklardan gelen mailler **hard fail** (direkt reddedilir)
- `~all` (soft fail) veya `?all` (nötr) kullanılmıyor — maksimum kısıtlama

**Test:**
```bash
dig +short TXT vendisens.com | grep spf
# Sonuç: "v=spf1 mx a:mail.vendisens.com ip4:167.86.70.187 -all"
```

---

### 2.2 DKIM (DomainKeys Identified Mail)

**Anahtar bilgileri:**
| Parametre | Değer |
|-----------|-------|
| Selector | `default` |
| Algoritma | RSA |
| Anahtar boyutu | **4096 bit** (endüstri standardının 2 katı) |
| Canonicalization | `relaxed/simple` |
| İmzalanan başlıklar | From (OversignHeaders) |

**DNS kaydı:**
```
default._domainkey.vendisens.com.  TXT  "v=DKIM1; h=sha256; k=rsa; p=MIICIjAN..."
```

**Sunucu konfigürasyonu:**
- **Servis:** opendkim (aktif, otomatik başlangıç)
- **Socket:** `/run/opendkim/opendkim.sock`
- **Anahtar dosyası:** `/etc/opendkim/keys/vendisens.com/default.private`
- **Postfix entegrasyonu:** `smtpd_milters = unix:/run/opendkim/opendkim.sock`

**Nasıl çalışır:**
1. Sunucudan çıkan her mail özel anahtar ile imzalanır
2. Alıcı, DNS'teki public key ile imzayı doğrular
3. İmza geçersizse DMARC politikası devreye girer

**Test:**
```bash
opendkim-testkey -d vendisens.com -s default -vvv
```

---

### 2.3 DMARC (Domain-based Message Authentication, Reporting & Conformance)

**DNS kaydı:**
```
_dmarc.vendisens.com.  TXT  "v=DMARC1; p=reject; adkim=s; aspf=s; pct=100;
  rua=mailto:dmarc-reports@vendisens.com;
  ruf=mailto:dmarc-failures@vendisens.com; fo=1"
```

**Politika ayrıntıları:**
| Parametre | Değer | Açıklama |
|-----------|-------|----------|
| `p=reject` | reject | SPF/DKIM başarısız → mail **reddedilir** |
| `adkim=s` | strict | DKIM selector domain From ile tam eşleşmeli |
| `aspf=s` | strict | SPF domaini From ile tam eşleşmeli |
| `pct=100` | %100 | Tüm mailler politikaya tabi |
| `fo=1` | her başarısızlık | Hem SPF hem DKIM başarısız için rapor gönder |
| `rua` | aggregate | Günlük özet raporları |
| `ruf` | forensic | Başarısız mail detayları |

> **Not:** DMARC raporları `dmarc-reports@vendisens.com` adresine gelir. Bu raporların düzenli
> incelenmesi önerilir (dmarcian.com, Postmark DMARC Digests gibi araçlarla).

---

### 2.4 MTA-STS (Mail Transfer Agent Strict Transport Security)

**DNS kaydı:**
```
_mta-sts.vendisens.com.  TXT  "v=STSv1; id=20260311000000"
```

**Policy dosyası** (`https://mta-sts.vendisens.com/.well-known/mta-sts.txt`):
```
version: STSv1
mode: enforce
mx: mail.vendisens.com
max_age: 604800
```

**Nasıl çalışır:**
- Başka mail sunucuları vendisens.com'a mail göndermeden önce bu politikayı kontrol eder
- `mode: enforce` → TLS olmadan bağlantı kesinlikle kabul edilmez
- Politika 7 gün (604800 saniye) önbelleğe alınır
- HTTPS üzerinden sunuluyor: Let's Encrypt sertifikası (`mta-sts.vendisens.com`)

**Policy ID güncellemesi:**
Politika değiştiğinde `_mta-sts` TXT kaydındaki `id=` değeri güncellenmelidir:
```bash
# Yeni ID oluştur (tarih+saat formatı)
date +%Y%m%d%H%M%S
```

---

### 2.5 TLS-RPT (TLS Reporting)

**DNS kaydı:**
```
_smtp._tls.vendisens.com.  TXT  "v=TLSRPTv1; rua=mailto:admin@manapconsulting.com"
```

Diğer mail sunucularının vendisens.com'a TLS bağlantısında yaşadığı sorunlar bu adrese raporlanır.

---

## 3. TLS / Şifreleme

### 3.1 Postfix TLS Yapılandırması

**Desteklenen protokoller:**
```
Aktif:  TLS 1.2, TLS 1.3
Devre dışı: SSLv2, SSLv3, TLS 1.0, TLS 1.1
```

**Konfigürasyon:**
```ini
# Protokol kısıtlaması
smtpd_tls_protocols           = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1
smtpd_tls_mandatory_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1
smtp_tls_protocols            = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1

# Cipher strength
smtpd_tls_ciphers           = high
smtpd_tls_mandatory_ciphers = high

# Zayıf algoritmalar devre dışı
smtpd_tls_exclude_ciphers = aNULL, eNULL, EXPORT, DES, 3DES,
                             RC2, RC4, MD5, PSK, SRP, DSS, SEED

# Port güvenlik seviyeleri
# Port 25  (MTA-MTA): opportunistic TLS (diğer sunucular desteklemeyebilir)
smtpd_tls_security_level = may
# Port 587 (Submission): STARTTLS zorunlu
# Port 465 (SMTPS): SSL/TLS wrapper zorunlu
```

**Port yapılandırması:**
| Port | Protokol | TLS | Kullanım |
|------|----------|-----|----------|
| 25 | SMTP | Opportunistic | MTA → MTA (gelen mail) |
| 587 | Submission | **Zorunlu** STARTTLS | İstemci → Sunucu |
| 465 | SMTPS | **Zorunlu** SSL/TLS | İstemci → Sunucu (eski) |

### 3.2 Dovecot TLS Yapılandırması

**Dosya:** `/etc/dovecot/conf.d/10-ssl.conf`

```ini
ssl_min_protocol = TLSv1.2

ssl_cipher_list = ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:\
  ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:\
  ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:\
  DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384

ssl_prefer_server_ciphers = yes
ssl_dh = </etc/dovecot/dh.pem   # 2048-bit DH parametreleri
```

**ECDHE (Elliptic Curve Diffie-Hellman Ephemeral)** öncelikli — her oturum için
yeni anahtar üretilir, **Perfect Forward Secrecy** sağlanır.

### 3.3 DH Parametreleri

```bash
# Mevcut DH parametresi
ls -la /etc/dovecot/dh.pem
# 2048-bit, openssl dhparam ile üretilmiş

# Yenilemek için (yavaş, arka planda çalıştır):
openssl dhparam -out /etc/dovecot/dh.pem 4096
systemctl restart dovecot
```

---

## 4. Kimlik Doğrulama

### 4.1 SASL Yapılandırması

```ini
# Postfix SASL (Dovecot backend)
smtpd_sasl_type             = dovecot
smtpd_sasl_path             = private/auth
smtpd_sasl_auth_enable      = yes
smtpd_sasl_security_options = noanonymous, noplaintext
# noplaintext: TLS olmadan plain text kimlik doğrulama YASAK
smtpd_sasl_tls_security_options = noanonymous
```

### 4.2 Sender Login Mismatch Koruması

```ini
smtpd_sender_login_maps   = mysql:/etc/postfix/mysql-sender-login-maps.cf
smtpd_sender_restrictions =
    reject_sender_login_mismatch,   # ← Kritik güvenlik önlemi
    permit_sasl_authenticated,
    permit_mynetworks,
    reject
```

**Ne sağlar:** Kimliği doğrulanmış bir kullanıcı yalnızca kendi mail adresinden
gönderebilir. Örn: `ahmet@vendisens.com` olarak giriş yapan kullanıcı
`fatma@vendisens.com` adından mail gönderemez.

---

## 5. Gelen Mail Filtreleme

### 5.1 Postscreen (Bot Filtresi)

Port 25'e bağlanan her IP, SMTP session açılmadan önce Postscreen tarafından test edilir.

```ini
postscreen_greet_action          = enforce    # Banner testi başarısız → bağlantı kesilir
postscreen_dnsbl_action          = enforce    # DNSBL eşiği aşılırsa → reddedilir
postscreen_blacklist_action      = drop       # Kara listede → bağlantı kesilir
postscreen_dnsbl_threshold       = 3          # 3+ puan = spam kaynağı

postscreen_dnsbl_sites =
    zen.spamhaus.org*3      # Spamhaus ZEN (en kapsamlı, 3 puan)
    bl.spamcop.net*2        # SpamCop (2 puan)
    b.barracudacentral.org*2 # Barracuda (2 puan)
```

**Puanlama örneği:**
- IP yalnızca Spamhaus'ta → 3 puan ≥ eşik → **reddedilir**
- IP yalnızca SpamCop'ta → 2 puan < eşik → geçer
- IP hem SpamCop hem Barracuda'da → 4 puan ≥ eşik → **reddedilir**

### 5.2 HELO Kısıtlamaları

```ini
smtpd_helo_required     = yes
smtpd_helo_restrictions =
    permit_mynetworks,
    reject_non_fqdn_helo_hostname,    # Geçersiz hostname → reddedilir
    reject_invalid_helo_hostname,     # Syntax hatası → reddedilir
    permit
```

### 5.3 Client Kısıtlamaları

```ini
smtpd_client_restrictions =
    permit_mynetworks,
    reject_unknown_reverse_client_hostname,  # PTR kaydı olmayan IP reddedilir
    permit
```

### 5.4 Gönderen Kısıtlamaları

```ini
smtpd_sender_restrictions =
    reject_non_fqdn_sender,           # FQDN olmayan gönderen → reddedilir
    reject_unknown_sender_domain,     # DNS'te olmayan domain → reddedilir
    reject_sender_login_mismatch,     # Kimlik uyumsuzluğu → reddedilir
    permit_sasl_authenticated,
    permit_mynetworks,
    reject
```

### 5.5 Alıcı Kısıtlamaları

```ini
smtpd_recipient_restrictions =
    reject_unauth_pipelining,          # Pipeline saldırısı önleme
    reject_non_fqdn_recipient,         # Geçersiz alıcı → reddedilir
    reject_unknown_recipient_domain,   # DNS'te olmayan domain → reddedilir
    permit_mynetworks,
    permit_sasl_authenticated,
    reject_unauth_destination,
    reject_rbl_client zen.spamhaus.org,   # RBL kontrolü (alıcı aşamasında)
    reject_rbl_client bl.spamcop.net,
    reject_rbl_client b.barracudacentral.org

smtpd_recipient_limit = 50   # Tek mailde maksimum 50 alıcı
```

---

## 6. Virüs ve Spam Tarama

### 6.1 Mail Akışı

```
Postfix (port 25)
        │
        │ content_filter = smtp-amavis:[127.0.0.1]:10024
        ▼
  Amavis (port 10024)
        │
        ├── ClamAV virüs taraması
        │       3.6M+ virüs imzası
        │       Günlük otomatik güncelleme
        │
        └── SpamAssassin spam skoru
                Çok katmanlı kural seti
                Bayesian filtresi
        │
        ▼ Temiz mail
  Postfix (port 10025)
        │
        ▼
  Dovecot → Kullanıcı posta kutusu
```

### 6.2 Virüs Tarama (ClamAV)

**Konfigürasyon dosyası:** `/etc/clamav/clamd.conf`

```ini
LocalSocket /var/run/clamav/clamd.ctl
User clamav
AllowSupplementaryGroups true
```

**Amavis entegrasyonu** (`/etc/amavis/conf.d/50-user`):
```perl
@av_scanners = (
  ['ClamAV-clamd',
    \&ask_daemon, ["CONTSCAN {}\n", "/var/run/clamav/clamd.ctl"],
    qr/\bOK$/m, qr/\bFOUND$/m, ...],
);
```

**Virüs bulunursa:** Mail `D_DISCARD` — sessizce silinir.
Gönderene bildirim gönderilmez (backscatter saldırısı önlemi).

**Veritabanı güncellemesi:**
```bash
# Manuel güncelleme
freshclam

# Otomatik: clamav-freshclam servisi her 12 saatte günceller
systemctl status clamav-freshclam
```

### 6.3 Spam Tarama (SpamAssassin + Amavis)

**Skor eşikleri:**
| Skor | Eylem |
|------|-------|
| ≥ 2.0 | `X-Spam-Status`, `X-Spam-Score` başlıkları eklenir |
| ≥ 6.31 | Konu satırına `*** SPAM ***` eklenir |
| ≥ 10.0 | Mail `D_DISCARD` — sessizce silinir |

**Kural güncellemesi:**
```bash
# Manuel
sa-update

# Otomatik: /etc/cron.daily/sa-update çalışır
cat /etc/cron.daily/sa-update
```

**Servis:**
```bash
systemctl status spamd     # Debian 12
# veya
systemctl status spamassassin  # Eski sistemler
```

---

## 7. Brute-Force ve DDoS Koruması

### 7.1 Postfix Rate Limiting

```ini
smtpd_client_connection_rate_limit      = 10   # IP başına 10 bağlantı/dakika
smtpd_client_message_rate_limit         = 20   # IP başına 20 mail/dakika
smtpd_client_recipient_rate_limit       = 50   # IP başına 50 alıcı/dakika
smtpd_client_connection_count_limit     = 10   # Eş zamanlı max 10 bağlantı
smtpd_client_new_tls_session_rate_limit = 10   # TLS handshake limiti

smtpd_error_sleep_time  = 5s    # Hatalı komut sonrası bekleme
smtpd_soft_error_limit  = 5     # 5 hatadan sonra yavaşlatma
smtpd_hard_error_limit  = 10    # 10 hatada bağlantı kesilir
```

### 7.2 Fail2ban

**Konfigürasyon:** `/etc/fail2ban/jail.d/mail.conf`

**Aktif jail'ler:**
| Jail | Koruduğu Servis | maxRetry | bantime |
|------|-----------------|----------|---------|
| `postfix` | SMTP gelen bağlantılar | 3 | **24 saat** |
| `postfix-sasl` | SASL kimlik doğrulama | 3 | **24 saat** |
| `postfix-rbl` | RBL başarısız bağlantılar | 1 | **24 saat** |
| `dovecot` | IMAP/POP3 girişleri | 3 | **24 saat** |

**Komutlar:**
```bash
# Aktif ban listesi
fail2ban-client status postfix-sasl

# IP ban'ı kaldır
fail2ban-client set postfix-sasl unbanip <IP>

# Tüm jail'leri görüntüle
fail2ban-client status
```

---

## 8. Güvenlik Duvarı

**Araç:** UFW (Uncomplicated Firewall)

**Kural tablosu:**
| Port | Protokol | Servis | Yön |
|------|----------|--------|-----|
| 22 | TCP | SSH | Gelen |
| 25 | TCP | SMTP (MTA) | Gelen |
| 80 | TCP | HTTP (certbot, MTA-STS) | Gelen |
| 143 | TCP | IMAP | Gelen |
| 443 | TCP | HTTPS | Gelen |
| 465 | TCP | SMTPS | Gelen |
| 587 | TCP | Submission | Gelen |
| 993 | TCP | IMAPS | Gelen |

**Varsayılan politika:**
- Gelen: **DENY** (listedeki portlar dışında her şey engellenir)
- Giden: **ALLOW** (tüm giden trafiğe izin verilir)

```bash
# Durum kontrolü
ufw status verbose

# Port ekleme (örnek)
ufw allow 8080/tcp comment 'Uygulama'
```

> **Uyarı:** Port 25'in kapatılması durumunda dışarıdan mail alınamaz.
> Port 587/465 kapatılırsa kullanıcılar mail gönderemez.

---

## 9. SSL Sertifika Yönetimi

### Mevcut Sertifikalar

| Domain | Yayınlayan | Son Kullanım | Kullanım |
|--------|-----------|--------------|----------|
| `mail.vendisens.com` | Let's Encrypt | 22 Mayıs 2026 | Postfix + Dovecot |
| `mta-sts.vendisens.com` | Let's Encrypt | 9 Haziran 2026 | MTA-STS policy |
| `vendisens.com` | Let's Encrypt | 18 Mayıs 2026 | Web |

### Otomatik Yenileme

Certbot zamanlayıcısı sertifikaları otomatik yeniler (30 gün önce):

```bash
# Zamanlayıcı durumu
systemctl status certbot.timer

# Manuel test (gerçekte yenilemez)
certbot renew --dry-run

# Tüm sertifikaları listele
certbot certificates
```

### Yenileme Sorunlarında Kontrol

```bash
# Nginx çalışıyor mu? (HTTP-01 challenge için gerekli)
systemctl status nginx

# Cloudflare proxy devre dışıysa (grey cloud) direct renewal:
certbot renew --nginx

# Cloudflare proxy aktifse port 80 erişimini test et:
curl -I http://mta-sts.vendisens.com/.well-known/acme-challenge/test
```

---

## 10. Dosya Sistemi Güvenliği

### Kritik Dosya İzinleri

| Dosya/Dizin | İzinler | Sahip | Açıklama |
|-------------|---------|-------|----------|
| `/etc/postfix/mysql-*.cf` | 640 | root:postfix | DB şifreleri |
| `/etc/dovecot/dovecot-sql.conf.ext` | 600 | root:root | DB şifreleri |
| `/etc/opendkim/keys/vendisens.com/default.private` | 600 | opendkim | DKIM özel anahtar |
| `/var/mail/vhosts/` | 770 | vmail:vmail | Posta kutuları |

```bash
# İzinleri kontrol et
ls -la /etc/postfix/mysql-*.cf
ls -la /etc/dovecot/dovecot-sql.conf.ext
ls -la /etc/opendkim/keys/vendisens.com/
```

### MySQL Güvenliği

- Anonim kullanıcılar kaldırıldı
- Uzaktan root girişi devre dışı
- Test veritabanı silindi
- `mailuser` → sadece `mailserver` veritabanına erişim yetkisi

---

## 11. İzleme ve Raporlama

### 11.1 DMARC Raporları

DMARC aggregate raporları günlük olarak `dmarc-reports@vendisens.com` adresine gönderilir.
Raporlar, SPF/DKIM doğrulaması başarısız olan mailleri içerir.

**Analiz araçları:**
- [dmarcian.com](https://dmarcian.com) — Ücretsiz DMARC rapor analizi
- [mxtoolbox.com/dmarc](https://mxtoolbox.com/dmarc.aspx)

### 11.2 TLS-RPT Raporları

`admin@manapconsulting.com` adresine günlük TLS bağlantı hata raporları gelir.

### 11.3 Fail2ban Aktivitesi

```bash
# Son banlanan IP'ler
fail2ban-client status postfix-sasl
fail2ban-client status dovecot

# Fail2ban logları
journalctl -u fail2ban -n 50 --no-pager
```

### 11.4 Mail Log Analizi

```bash
# Son gelen maillerin durumu (Postfix)
tail -f /var/log/mail.log

# Amavis virüs/spam yakaladıklarını gör
grep -i "INFECTED\|SPAM\|BOUNCE" /var/log/mail.log | tail -20

# Reddedilen bağlantılar
grep "NOQUEUE\|reject" /var/log/mail.log | tail -20
```

---

## 12. Bakım Prosedürleri

### 12.1 Günlük Kontroller (Otomatik)

| Görev | Araç | Zamanlama |
|-------|------|-----------|
| ClamAV imza güncellemesi | clamav-freshclam | Her 12 saat |
| SpamAssassin kural güncellemesi | sa-update cron | Günlük |
| Certbot yenileme kontrolü | certbot.timer | Günlük |

### 12.2 Haftalık Kontroller (Manuel)

```bash
# Servis durumları
systemctl is-active postfix dovecot opendkim fail2ban clamav-daemon amavis spamd

# Disk kullanımı (posta kutuları)
du -sh /var/mail/vhosts/*

# Fail2ban istatistikleri
fail2ban-client status

# ClamAV veritabanı yaşı
clamscan --version
```

### 12.3 Aylık Kontroller

```bash
# Sertifika süresi
certbot certificates | grep "Expiry Date"

# DKIM anahtar rotasyonu (yılda bir önerilir)
# 1. Yeni anahtar üret:
opendkim-genkey -D /etc/opendkim/keys/vendisens.com/ \
  -d vendisens.com -s default2 --bits=4096
# 2. DNS'e default2._domainkey kaydı ekle
# 3. opendkim KeyTable/SigningTable güncelle
# 4. Eski kayıt yayıldıktan sonra default._domainkey kaldır

# MySQL slow query kontrolü
mysql -e "SHOW STATUS LIKE 'Slow_queries';"
```

### 12.4 Acil Durum Prosedürleri

**Servis çöktüğünde:**
```bash
# Postfix
systemctl restart postfix
postfix check
tail -20 /var/log/mail.log

# Dovecot
systemctl restart dovecot
doveadm log errors

# Amavis
systemctl restart amavis
journalctl -u amavis -n 20

# ClamAV
systemctl restart clamav-daemon
```

**IP'yi acil ban'dan kaldır:**
```bash
fail2ban-client set postfix-sasl unbanip <IP>
fail2ban-client set dovecot unbanip <IP>
```

**Virüslü mail karantinaya aldıktan sonra:**
```bash
# Karantina dizini
ls /var/lib/amavis/virusmails/
```

---

## 13. Konfigürasyon Referansı

### Dosya Lokasyonları

| Servis | Konfigürasyon |
|--------|---------------|
| Postfix ana config | `/etc/postfix/main.cf` |
| Postfix servis config | `/etc/postfix/master.cf` |
| Dovecot ana config | `/etc/dovecot/dovecot.conf` |
| Dovecot SSL | `/etc/dovecot/conf.d/10-ssl.conf` |
| Dovecot SQL | `/etc/dovecot/dovecot-sql.conf.ext` |
| Amavis kullanıcı config | `/etc/amavis/conf.d/50-user` |
| Amavis içerik filtresi | `/etc/amavis/conf.d/15-content_filter_mode` |
| ClamAV daemon | `/etc/clamav/clamd.conf` |
| opendkim | `/etc/opendkim.conf` |
| opendkim anahtarlar | `/etc/opendkim/keys/vendisens.com/` |
| Fail2ban mail kuralları | `/etc/fail2ban/jail.d/mail.conf` |
| MTA-STS policy | `/var/www/mta-sts-vendisens/.well-known/mta-sts.txt` |
| MTA-STS nginx | `/etc/nginx/sites-enabled/mta-sts.vendisens.com.conf` |

### Kritik Servisler

```bash
# Tüm mail servislerinin durumu
for s in postfix dovecot opendkim amavis clamav-daemon clamav-freshclam fail2ban spamd nginx; do
  printf "%-20s %s\n" "$s" "$(systemctl is-active $s 2>/dev/null)"
done
```

### Hızlı Test Komutları

```bash
# Mail teslimatını test et (harici araç)
# https://www.mail-tester.com

# SPF kontrolü
dig +short TXT vendisens.com | grep spf

# DKIM DNS kontrolü
dig +short TXT default._domainkey.vendisens.com

# DMARC kontrolü
dig +short TXT _dmarc.vendisens.com

# MTA-STS policy erişimi
curl -s https://mta-sts.vendisens.com/.well-known/mta-sts.txt

# DKIM anahtar doğrulama
opendkim-testkey -d vendisens.com -s default

# TLS bağlantı testi (Postfix)
openssl s_client -connect mail.vendisens.com:587 -starttls smtp

# TLS bağlantı testi (Dovecot IMAP)
openssl s_client -connect mail.vendisens.com:993
```

---

*Bu belge son olarak 11 Mart 2026 tarihinde güncellenmiştir.*
*Güvenlik açığı bildirimleri için: security@manapconsulting.com*
