# Mail Server Hardening Guide

Complete hardening guide for the mail server stack. Based on analysis of the live server configuration.

## Quick Start

After running `install.sh`, apply all hardening settings with:

```bash
sudo ./scripts/harden.sh
```

This script automates all steps below.

---

## Hardening Checklist

```
CRITICAL
├── [ ] TLS minimum version → TLS 1.2
├── [ ] Weak cipher suites disabled
├── [ ] Rate limiting enabled
└── [ ] Firewall (ufw) configured

HIGH
├── [ ] Fail2ban bantime → 24h
├── [ ] HELO restrictions
├── [ ] Recipient/sender restrictions strengthened
└── [ ] Postscreen bot filter

MEDIUM
├── [ ] SpamAssassin enabled
├── [ ] DKIM configured
├── [ ] DH parameters generated
├── [ ] MySQL hardening
└── [ ] DMARC policy → quarantine/reject

ONGOING
├── [ ] Monitor logs daily
├── [ ] Review fail2ban bans weekly
└── [ ] Update DNS records (DMARC progression)
```

---

## 1. TLS Hardening

### Problem
Default configuration accepts TLS 1.0 and 1.1, which are deprecated and have known vulnerabilities (POODLE, BEAST).

### Fix — Postfix

```bash
postconf -e "smtpd_tls_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtpd_tls_mandatory_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtp_tls_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtpd_tls_ciphers = high"
postconf -e "smtpd_tls_mandatory_ciphers = high"
postconf -e "smtpd_tls_exclude_ciphers = aNULL, eNULL, EXPORT, DES, 3DES, RC2, RC4, MD5, PSK, SRP, DSS, SEED"
postfix reload
```

### Fix — Dovecot

```bash
# /etc/dovecot/conf.d/10-ssl.conf
ssl_min_protocol = TLSv1.2
ssl_cipher_list = ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305
ssl_prefer_server_ciphers = yes

# Generate strong DH params (run once)
openssl dhparam -out /etc/dovecot/dh.pem 2048
# Then add to 10-ssl.conf:
ssl_dh = </etc/dovecot/dh.pem
```

### Verify

```bash
# Check Postfix TLS
openssl s_client -starttls smtp -connect localhost:587 2>&1 | grep -E 'Protocol|Cipher'
# Expected: TLSv1.2 or TLSv1.3

# Check Dovecot TLS
openssl s_client -connect localhost:993 2>&1 | grep -E 'Protocol|Cipher'

# SSL Labs test (external)
# https://www.ssllabs.com/ssltest/
```

---

## 2. Rate Limiting

### Problem
All `smtpd_client_*_rate_limit` values are `0` (unlimited). Allows brute-force and spam relay attempts.

### Fix

```bash
postconf -e "smtpd_client_connection_rate_limit = 10"
postconf -e "smtpd_client_message_rate_limit = 20"
postconf -e "smtpd_client_recipient_rate_limit = 50"
postconf -e "smtpd_client_connection_count_limit = 10"
postconf -e "smtpd_error_sleep_time = 5s"
postconf -e "smtpd_soft_error_limit = 5"
postconf -e "smtpd_hard_error_limit = 10"
postfix reload
```

### Explanation

```
smtpd_client_connection_rate_limit = 10
   └── Max 10 new connections per client per minute

smtpd_client_message_rate_limit = 20
   └── Max 20 messages per client per minute

smtpd_error_sleep_time = 5s
   └── Wait 5s between each error response (slows brute-force)

smtpd_hard_error_limit = 10
   └── Disconnect client after 10 protocol errors
```

---

## 3. HELO Restrictions

### Problem
No HELO validation. Spammers often send invalid or missing HELO.

### Fix

```bash
# /etc/postfix/main.cf
smtpd_helo_required = yes
smtpd_helo_restrictions =
    permit_mynetworks,
    reject_non_fqdn_helo_hostname,
    reject_invalid_helo_hostname,
    permit
```

---

## 4. Postscreen (Bot Filter)

### Problem
No Postscreen. All connections, including bot spam, reach the full SMTP daemon.

### How It Works

```
Internet Bot
     │
     ▼ Port 25
┌────────────────┐
│  POSTSCREEN    │  ← Fast pre-filter
│                │    - DNSBL check
│  Bot detected? │    - Greeting banner trap
│  YES → DROP    │    - Pipeline check
│  NO  → pass    │
└────────┬───────┘
         │ legitimate connection only
         ▼
┌────────────────┐
│    SMTPD       │  ← Full SMTP processing
└────────────────┘
```

### Fix — master.cf

```
# Replace:
smtp      inet  n - n - - smtpd

# With:
smtp      inet  n - n - 1  postscreen
smtpd     pass  - - n - -  smtpd
dnsblog   unix  - - n - 0  dnsblog
tlsproxy  unix  - - n - 0  tlsproxy
```

### Fix — main.cf

```
postscreen_access_list      = permit_mynetworks
postscreen_blacklist_action = drop
postscreen_greet_action     = enforce
postscreen_dnsbl_action     = enforce
postscreen_dnsbl_threshold  = 3
postscreen_dnsbl_sites      =
    zen.spamhaus.org*3
    bl.spamcop.net*2
    b.barracudacentral.org*2
```

---

## 5. Firewall (ufw)

### Problem
`ufw` is not installed. No application-level firewall rules.

### Fix

```bash
apt install -y ufw

ufw default deny incoming
ufw default allow outgoing

ufw allow 22/tcp    # SSH
ufw allow 25/tcp    # SMTP
ufw allow 587/tcp   # Submission
ufw allow 465/tcp   # SMTPS
ufw allow 143/tcp   # IMAP
ufw allow 993/tcp   # IMAPS
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS

ufw enable
ufw status verbose
```

---

## 6. Fail2ban Tuning

### Problem
Current ban time is 1 hour. Attackers wait it out and retry.

### Fix

```bash
# /etc/fail2ban/jail.d/mail.conf
[postfix]
bantime  = 86400   # 24 hours
maxretry = 3

[postfix-sasl]
bantime  = 86400
maxretry = 3

[dovecot]
bantime  = 86400
maxretry = 3
```

```bash
# Apply changes
systemctl restart fail2ban

# Monitor
fail2ban-client status
fail2ban-client status postfix
```

---

## 7. DKIM Setup

### Install

```bash
apt install -y opendkim opendkim-tools

# Generate key pair
mkdir -p /etc/opendkim/keys/example.com
opendkim-genkey -D /etc/opendkim/keys/example.com/ -d example.com -s mail
chown -R opendkim:opendkim /etc/opendkim
chmod 600 /etc/opendkim/keys/example.com/mail.private
```

### DNS Record

```bash
# View the DNS record to add
cat /etc/opendkim/keys/example.com/mail.txt
# Add this as TXT record: mail._domainkey.example.com
```

### Connect to Postfix

```bash
# /etc/postfix/main.cf
smtpd_milters     = unix:/run/opendkim/opendkim.sock
non_smtpd_milters = $smtpd_milters
milter_default_action = accept
```

### Verify

```bash
# Send a test email then check headers for DKIM-Signature
# Or use: https://www.mail-tester.com
```

---

## 8. DMARC Progression

Move from monitoring to enforcement gradually:

```
Week 1-2: Monitor only
  _dmarc.example.com TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"

Week 3-4: Quarantine (after reviewing reports)
  _dmarc.example.com TXT "v=DMARC1; p=quarantine; pct=25; rua=mailto:dmarc@example.com"

Month 2+: Full enforcement
  _dmarc.example.com TXT "v=DMARC1; p=reject; rua=mailto:dmarc@example.com"
```

Free DMARC report analysis: https://dmarcian.com

---

## 9. SpamAssassin

### Fix

```bash
apt install -y spamassassin spamc
systemctl enable --now spamassassin

# Keep rules up to date
echo '#!/bin/bash
sa-update && systemctl reload spamassassin' > /etc/cron.daily/sa-update
chmod +x /etc/cron.daily/sa-update
```

---

## 10. MySQL Hardening

```bash
# Remove anonymous users
mysql -u root -p -e "DELETE FROM mysql.user WHERE User='';"

# Remove remote root access
mysql -u root -p -e "DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost','127.0.0.1','::1');"

# Remove test database
mysql -u root -p -e "DROP DATABASE IF EXISTS test;"

# Apply changes
mysql -u root -p -e "FLUSH PRIVILEGES;"

# Bind MySQL to localhost only
# /etc/mysql/mariadb.conf.d/50-server.cnf
# bind-address = 127.0.0.1
```

---

## Monitoring

### Daily Log Check

```bash
# Authentication failures
grep "authentication failure\|SASL.*failed" /var/log/mail.log | tail -20

# Blocked by Postscreen
grep "DNSBL rank" /var/log/mail.log | tail -20

# Fail2ban bans
fail2ban-client status postfix-sasl
```

### Test Mail Reputation

After applying hardening, test with:

- **[mail-tester.com](https://www.mail-tester.com)** — send an email, get a score
- **[MXToolbox](https://mxtoolbox.com/SuperTool.aspx)** — check blacklists, DNS, SMTP
- **[SSL Labs](https://www.ssllabs.com/ssltest/)** — TLS configuration grade
- **[DMARC Analyzer](https://dmarcian.com/dmarc-inspector/)** — check DMARC record

### Target: 10/10 on mail-tester.com

```
Checklist for perfect score:
├── [x] Valid SPF record
├── [x] DKIM signature
├── [x] DMARC record
├── [x] Reverse DNS (PTR) matches hostname
├── [x] Not on any blacklist
├── [x] Valid MX record
└── [x] Clean HTML (no spam triggers)
```
