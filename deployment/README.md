# Mail Server Deployment Configuration

Complete deployment guide for the mail system with Postfix, Dovecot, and web applications.

## Table of Contents

- [System Architecture](#system-architecture)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Installation](#detailed-installation)
- [Configuration Guide](#configuration-guide)
- [Security Setup](#security-setup)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Maintenance](#maintenance)

---

## System Architecture

### Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         INTERNET                                 │
└────────────┬────────────────────────────────────────────────────┘
             │
             │ Port 25 (SMTP), 587 (Submission), 465 (SMTPS)
             │ Port 143 (IMAP), 993 (IMAPS)
             │ Port 80 (HTTP), 443 (HTTPS)
             │
┌────────────▼────────────────────────────────────────────────────┐
│                       FIREWALL (ufw/iptables)                    │
└────────────┬────────────────────────────────────────────────────┘
             │
    ┌────────┴────────┐
    │                 │
┌───▼────┐      ┌────▼─────┐
│ NGINX  │      │ POSTFIX  │
│(Proxy) │      │  (MTA)   │
└───┬────┘      └────┬─────┘
    │                │
    │                │ LMTP (Local Mail Transfer)
    │                │
    │           ┌────▼─────┐
    │           │ DOVECOT  │
    │           │ (IMAP/   │
    │           │  LMTP)   │
    │           └────┬─────┘
    │                │
┌───┴────────────────┴─────┐
│   WEB APPLICATIONS        │
│                           │
│  ┌──────────────────┐    │
│  │   Mail Admin     │    │
│  │   (Port 8080)    │    │
│  └──────────┬───────┘    │
│             │             │
│  ┌──────────▼───────┐    │
│  │   Groupware      │    │
│  │   (Port 8081)    │    │
│  │  - CalDAV/IMAP   │    │
│  └──────────┬───────┘    │
│             │             │
│  ┌──────────▼───────┐    │
│  │    Webmail       │    │
│  │   (Port 8082)    │    │
│  └──────────┬───────┘    │
└─────────────┼────────────┘
              │
         ┌────▼─────┐
         │  MySQL   │
         │ Database │
         └──────────┘
```

### Mail Flow Diagram

#### Incoming Mail

```
Internet
   │
   │ SMTP (Port 25)
   ▼
┌──────────────┐
│   POSTFIX    │
│              │
│ 1. Receive   │
│ 2. Check     │──────► SpamAssassin (optional)
│    - Domain  │
│    - User    │──────► MySQL Lookup
│ 3. Accept    │
└──────┬───────┘
       │ LMTP
       ▼
┌──────────────┐
│   DOVECOT    │
│              │
│ 1. Store     │
│ 2. Index     │
│ 3. Deliver   │
└──────┬───────┘
       │
       ▼
┌──────────────────────┐
│ /var/mail/vhosts/    │
│   domain.com/        │
│     user/            │
│       Maildir/       │
│         new/         │
│         cur/         │
│         tmp/         │
└──────────────────────┘
```

#### Outgoing Mail (User Sending)

```
┌──────────────┐
│ Mail Client  │
│ (Thunderbird,│
│  Outlook,    │
│  Webmail)    │
└──────┬───────┘
       │ SMTP AUTH (Port 587/465)
       │ Username: user@domain.com
       │ Password: ********
       ▼
┌──────────────┐
│   POSTFIX    │
│              │
│ 1. Auth via  │◄──── SASL
│    Dovecot   │
│              │
│ 2. Check     │◄──── MySQL
│    Sender    │      (sender_permissions)
│              │
│ 3. Accept &  │
│    Queue     │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   INTERNET   │
│  (External   │
│   Mail       │
│   Servers)   │
└──────────────┘
```

### Database Schema

```
┌─────────────────────────────────────────────────────────────┐
│                      MYSQL DATABASE                          │
│                      (mailserver)                            │
│                                                              │
│  ┌──────────────────┐         ┌─────────────────────┐      │
│  │ virtual_domains  │         │  virtual_users      │      │
│  ├──────────────────┤         ├─────────────────────┤      │
│  │ id (PK)          │◄────┐   │ id (PK)             │      │
│  │ name             │     │   │ domain_id (FK)      │      │
│  │ webmail_enabled  │     └───│ email               │      │
│  │ caldav_enabled   │         │ password (SHA512)   │      │
│  │ carddav_enabled  │         └─────────────────────┘      │
│  └──────────────────┘                                       │
│           ▲                                                 │
│           │                                                 │
│           │                    ┌─────────────────────┐     │
│           │                    │ virtual_aliases     │     │
│           │                    ├─────────────────────┤     │
│           │                    │ id (PK)             │     │
│           └────────────────────│ domain_id (FK)      │     │
│                                │ source              │     │
│                                │ destination         │     │
│                                └─────────────────────┘     │
│                                                             │
│  ┌──────────────────┐         ┌─────────────────────┐     │
│  │ calendars        │         │  events             │     │
│  ├──────────────────┤         ├─────────────────────┤     │
│  │ id (PK)          │◄────────│ calendar_id (FK)    │     │
│  │ user_email       │         │ title               │     │
│  │ name             │         │ start_time          │     │
│  │ color            │         │ end_time            │     │
│  └──────────────────┘         │ location            │     │
│                                │ description         │     │
│                                └─────────────────────┘     │
│                                                             │
│  ┌──────────────────┐         ┌─────────────────────┐     │
│  │ addressbooks     │         │  contacts           │     │
│  ├──────────────────┤         ├─────────────────────┤     │
│  │ id (PK)          │◄────────│ addressbook_id (FK) │     │
│  │ user_email       │         │ name                │     │
│  │ name             │         │ email               │     │
│  └──────────────────┘         │ phone               │     │
│                                │ vcard_data          │     │
│                                └─────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

### System Requirements

- **OS**: Debian 11/12 or Ubuntu 20.04/22.04 LTS (64-bit)
- **RAM**: Minimum 2GB (4GB+ recommended for production)
- **Disk**: 20GB+ free space
- **CPU**: 2+ cores recommended
- **Network**: Public IP address with reverse DNS (PTR record)

### Required Software

- MySQL/MariaDB 10.5+
- Postfix 3.5+
- Dovecot 2.3+
- Go 1.19+
- Node.js 16+
- Certbot (Let's Encrypt)

### DNS Requirements

Before installation, configure these DNS records:

```
; A Record for mail server
mail.example.com.        IN A     203.0.113.10

; MX Record
example.com.             IN MX 10 mail.example.com.

; SPF Record
example.com.             IN TXT   "v=spf1 mx ~all"

; DMARC Record
_dmarc.example.com.      IN TXT   "v=DMARC1; p=none; rua=mailto:postmaster@example.com"

; Reverse DNS (PTR) - Configure with your hosting provider
10.113.0.203.in-addr.arpa. IN PTR mail.example.com.
```

### Firewall Ports

```
┌─────────────┬─────────────────────────────────────────┐
│    Port     │              Purpose                    │
├─────────────┼─────────────────────────────────────────┤
│ 25/tcp      │ SMTP - Incoming mail                    │
│ 587/tcp     │ Submission - Authenticated sending      │
│ 465/tcp     │ SMTPS - Secure SMTP                     │
│ 143/tcp     │ IMAP - Email access                     │
│ 993/tcp     │ IMAPS - Secure IMAP                     │
│ 80/tcp      │ HTTP - Web & Let's Encrypt              │
│ 443/tcp     │ HTTPS - Secure web access               │
│ 3306/tcp    │ MySQL - localhost only (not exposed)    │
└─────────────┴─────────────────────────────────────────┘
```

---

## Quick Start

```bash
# 1. Clone repository
git clone https://github.com/manapconsulting/groupware-suite.git
cd groupware-suite/deployment

# 2. Review configuration templates
ls -la postfix/
ls -la dovecot/

# 3. Run automated installation
sudo ./scripts/install.sh

# 4. Follow prompts to enter:
#    - Hostname (mail.example.com)
#    - Domain (example.com)
#    - MySQL passwords
#    - Email for SSL certificates

# 5. Installation will:
#    ✓ Install packages
#    ✓ Configure MySQL
#    ✓ Setup Postfix & Dovecot
#    ✓ Obtain SSL certificates
#    ✓ Create mail directories
#    ✓ Start services
```

---

## Detailed Installation

### Step 1: System Preparation

```bash
# Update system
apt update && apt upgrade -y

# Set hostname
hostnamectl set-hostname mail.example.com

# Verify hostname
hostname -f
# Should output: mail.example.com

# Configure /etc/hosts
cat >> /etc/hosts << EOF
203.0.113.10 mail.example.com mail
EOF
```

### Step 2: Install Required Packages

```bash
# Install mail server components
apt install -y \
  postfix \
  postfix-mysql \
  dovecot-core \
  dovecot-imapd \
  dovecot-lmtpd \
  dovecot-mysql \
  mariadb-server \
  certbot \
  ufw

# Install development tools
apt install -y \
  build-essential \
  golang-go \
  nodejs \
  npm

# Verify installations
postconf mail_version
doveconf -n | head -1
mysql --version
go version
node --version
```

### Step 3: MySQL Database Setup

```bash
# Secure MySQL installation
mysql_secure_installation
```

Answer prompts:
```
Set root password? [Y/n] Y
Remove anonymous users? [Y/n] Y
Disallow root login remotely? [Y/n] Y
Remove test database? [Y/n] Y
Reload privilege tables? [Y/n] Y
```

Create database and user:

```bash
# Generate secure password
DB_PASSWORD=$(openssl rand -base64 32)
echo "Database Password: $DB_PASSWORD" > /root/mail_credentials.txt
chmod 600 /root/mail_credentials.txt

# Create database
mysql -u root -p << EOF
CREATE DATABASE mailserver CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE USER 'mailuser'@'localhost' IDENTIFIED BY '$DB_PASSWORD';
GRANT ALL PRIVILEGES ON mailserver.* TO 'mailuser'@'localhost';
FLUSH PRIVILEGES;
EOF

# Import schema
mysql -u root -p mailserver < database/schema.sql

# Verify tables
mysql -u root -p -e "USE mailserver; SHOW TABLES;"
```

Expected output:
```
+-------------------------+
| Tables_in_mailserver    |
+-------------------------+
| addressbooks            |
| calendar_members        |
| calendars               |
| contacts                |
| events                  |
| sender_permissions      |
| shared_calendars        |
| shared_events           |
| virtual_aliases         |
| virtual_domains         |
| virtual_users           |
+-------------------------+
```

### Step 4: Configure Postfix

#### 4.1 Update Configuration Files

```bash
# Backup original configuration
cp /etc/postfix/main.cf /etc/postfix/main.cf.orig
cp /etc/postfix/master.cf /etc/postfix/master.cf.orig

# Copy template files
cp postfix/main.cf /etc/postfix/main.cf
cp postfix/master.cf /etc/postfix/master.cf
cp postfix/mysql-*.cf /etc/postfix/

# Replace placeholders
HOSTNAME="mail.example.com"
sed -i "s/CHANGE_ME_HOSTNAME/$HOSTNAME/g" /etc/postfix/main.cf
sed -i "s/CHANGE_ME_DB_PASSWORD/$DB_PASSWORD/g" /etc/postfix/mysql-*.cf

# Set permissions
chmod 640 /etc/postfix/mysql-*.cf
chown root:postfix /etc/postfix/mysql-*.cf

# Update mailname
echo "example.com" > /etc/mailname
```

#### 4.2 Test MySQL Lookups

```bash
# First, add a test domain and user
mysql -u mailuser -p"$DB_PASSWORD" mailserver << EOF
INSERT INTO virtual_domains (name) VALUES ('example.com');
INSERT INTO virtual_users (domain_id, email, password)
VALUES (1, 'test@example.com', 'temp_password');
EOF

# Test domain lookup
postmap -q "example.com" mysql:/etc/postfix/mysql-virtual-domains.cf
# Expected: example.com

# Test user lookup
postmap -q "test@example.com" mysql:/etc/postfix/mysql-virtual-users.cf
# Expected: /var/mail/vhosts/example.com/test/Maildir

# Test alias lookup (should be empty for now)
postmap -q "alias@example.com" mysql:/etc/postfix/mysql-virtual-aliases.cf
```

#### 4.3 Verify Postfix Configuration

```bash
# Check configuration syntax
postfix check

# View active configuration
postconf -n

# Test SMTP connectivity
telnet localhost 25
# Type: EHLO localhost
# Type: QUIT
```

### Step 5: Configure Dovecot

#### 5.1 Update Configuration Files

```bash
# Backup original configuration
cp /etc/dovecot/dovecot.conf /etc/dovecot/dovecot.conf.orig
tar czf /root/dovecot-conf-backup.tar.gz /etc/dovecot/conf.d/

# Copy template files
cp dovecot/dovecot.conf /etc/dovecot/
cp dovecot/dovecot-sql.conf.ext /etc/dovecot/
cp dovecot/auth-sql.conf.ext /etc/dovecot/conf.d/
cp dovecot/10-*.conf /etc/dovecot/conf.d/
cp dovecot/90-*.conf /etc/dovecot/conf.d/

# Replace placeholders
sed -i "s/CHANGE_ME_HOSTNAME/$HOSTNAME/g" /etc/dovecot/conf.d/10-ssl.conf
sed -i "s/CHANGE_ME_DB_PASSWORD/$DB_PASSWORD/g" /etc/dovecot/dovecot-sql.conf.ext

# Set permissions
chmod 600 /etc/dovecot/dovecot-sql.conf.ext
chown root:root /etc/dovecot/dovecot-sql.conf.ext
```

#### 5.2 Create Virtual Mail User

```bash
# Create vmail user and group
groupadd -g 5000 vmail
useradd -g vmail -u 5000 vmail -d /var/mail -s /usr/sbin/nologin

# Create mail directory structure
mkdir -p /var/mail/vhosts
chown -R vmail:vmail /var/mail
chmod -R 770 /var/mail/vhosts

# Verify
ls -la /var/mail/
id vmail
```

Expected output:
```
uid=5000(vmail) gid=5000(vmail) groups=5000(vmail)
```

#### 5.3 Test Dovecot Configuration

```bash
# Check configuration
doveconf -n

# Test SQL connection
doveconf -a | grep -A 10 "sql"
```

### Step 6: Obtain SSL Certificates

#### 6.1 Stop Services for Certificate Acquisition

```bash
# Stop services temporarily
systemctl stop postfix dovecot nginx 2>/dev/null

# Verify ports are free
ss -tlnp | grep -E ':80|:443'
```

#### 6.2 Obtain Certificates

```bash
# Get certificate for main domain
certbot certonly \
  --standalone \
  -d mail.example.com \
  --email admin@example.com \
  --agree-tos \
  --non-interactive

# Verify certificate
ls -la /etc/letsencrypt/live/mail.example.com/
```

Expected files:
```
cert.pem -> ../../archive/mail.example.com/cert1.pem
chain.pem -> ../../archive/mail.example.com/chain1.pem
fullchain.pem -> ../../archive/mail.example.com/fullchain1.pem
privkey.pem -> ../../archive/mail.example.com/privkey1.pem
```

#### 6.3 Setup Auto-Renewal

```bash
# Test renewal
certbot renew --dry-run

# Create renewal hook
cat > /etc/letsencrypt/renewal-hooks/deploy/reload-mail.sh << 'EOF'
#!/bin/bash
systemctl reload postfix dovecot
EOF

chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-mail.sh

# Check cron job
systemctl status certbot.timer
```

### Step 7: Start Services

```bash
# Enable services
systemctl enable postfix dovecot

# Start services
systemctl start postfix dovecot

# Check status
systemctl status postfix
systemctl status dovecot

# Check ports
ss -tlnp | grep -E 'postfix|dovecot'
```

Expected output:
```
LISTEN  0  100  0.0.0.0:25    0.0.0.0:*  users:(("master",pid=1234))
LISTEN  0  100  0.0.0.0:587   0.0.0.0:*  users:(("master",pid=1234))
LISTEN  0  100  0.0.0.0:143   0.0.0.0:*  users:(("dovecot",pid=5678))
LISTEN  0  100  0.0.0.0:993   0.0.0.0:*  users:(("dovecot",pid=5678))
```

### Step 8: Create First User

```bash
# Generate password hash
PASSWORD_HASH=$(doveadm pw -s SHA512-CRYPT -p 'SecurePassword123')

# Add domain and user
mysql -u mailuser -p"$DB_PASSWORD" mailserver << EOF
-- Clear test data if any
DELETE FROM virtual_users;
DELETE FROM virtual_domains;

-- Add your domain
INSERT INTO virtual_domains (name, webmail_enabled, caldav_enabled, carddav_enabled)
VALUES ('example.com', 1, 1, 1);

-- Add your user
INSERT INTO virtual_users (domain_id, email, password)
VALUES (1, 'admin@example.com', '$PASSWORD_HASH');
EOF

# Create mailbox directory
mkdir -p /var/mail/vhosts/example.com/admin/Maildir
chown -R vmail:vmail /var/mail/vhosts/example.com
```

### Step 9: Configure Firewall

```bash
# Enable UFW
ufw --force enable

# Allow SSH (important - don't lock yourself out!)
ufw allow 22/tcp

# Allow mail ports
ufw allow 25/tcp    comment 'SMTP'
ufw allow 587/tcp   comment 'Submission'
ufw allow 465/tcp   comment 'SMTPS'
ufw allow 143/tcp   comment 'IMAP'
ufw allow 993/tcp   comment 'IMAPS'

# Allow web ports
ufw allow 80/tcp    comment 'HTTP'
ufw allow 443/tcp   comment 'HTTPS'

# Check status
ufw status numbered
```

### Step 10: Test Mail System

#### Test Authentication

```bash
# Test Dovecot authentication
doveadm auth test admin@example.com 'SecurePassword123'
```

Expected output:
```
passdb: admin@example.com auth succeeded
extra fields:
  user=admin@example.com
```

#### Test SMTP

```bash
# Test SMTP connection
telnet localhost 25

# Commands to type:
EHLO localhost
MAIL FROM:<admin@example.com>
RCPT TO:<admin@example.com>
DATA
Subject: Test Email
This is a test.
.
QUIT
```

#### Test IMAP

```bash
# Test IMAP connection
telnet localhost 143

# Commands to type:
a1 LOGIN admin@example.com SecurePassword123
a2 LIST "" "*"
a3 LOGOUT
```

#### Send Test Email

```bash
# Install mail utils
apt install -y mailutils

# Send test email
echo "This is a test email from the mail server." | \
  mail -s "Test Email" \
  -a "From: admin@example.com" \
  admin@example.com

# Check mailbox
ls -la /var/mail/vhosts/example.com/admin/Maildir/new/
```

---

## Configuration Guide

### Adding More Domains

```sql
-- Add new domain
INSERT INTO virtual_domains (name, webmail_enabled, caldav_enabled, carddav_enabled)
VALUES ('newdomain.com', 1, 1, 1);

-- Add user for new domain
INSERT INTO virtual_users (domain_id, email, password)
VALUES (
  (SELECT id FROM virtual_domains WHERE name='newdomain.com'),
  'user@newdomain.com',
  'PASSWORD_HASH_HERE'
);
```

Then update DNS and obtain SSL certificate:

```bash
# Update DNS first, then:
certbot certonly --standalone -d mail.newdomain.com
```

Update Dovecot SNI configuration:

```bash
# Edit /etc/dovecot/conf.d/10-ssl.conf
cat >> /etc/dovecot/conf.d/10-ssl.conf << EOF

local_name mail.newdomain.com {
  ssl_cert = </etc/letsencrypt/live/mail.newdomain.com/fullchain.pem
  ssl_key = </etc/letsencrypt/live/mail.newdomain.com/privkey.pem
}
EOF

systemctl reload dovecot
```

### Adding Email Aliases

```sql
-- Add alias
INSERT INTO virtual_aliases (domain_id, source, destination)
VALUES (
  (SELECT id FROM virtual_domains WHERE name='example.com'),
  'sales@example.com',
  'admin@example.com'
);

-- Add catch-all alias (forward all unmatched to one account)
INSERT INTO virtual_aliases (domain_id, source, destination)
VALUES (
  (SELECT id FROM virtual_domains WHERE name='example.com'),
  '@example.com',
  'admin@example.com'
);
```

### Sender Permissions (Prevent Spoofing)

```sql
-- Allow user to send from specific addresses
INSERT INTO sender_permissions (user_email, allowed_sender)
VALUES
  ('admin@example.com', 'admin@example.com'),
  ('admin@example.com', 'noreply@example.com'),
  ('admin@example.com', 'support@example.com');
```

---

## Security Setup

### 1. DKIM Configuration (Optional but Recommended)

```bash
# Install OpenDKIM
apt install -y opendkim opendkim-tools

# Generate keys
mkdir -p /etc/opendkim/keys/example.com
opendkim-genkey -D /etc/opendkim/keys/example.com/ \
  -d example.com \
  -s mail

# Set permissions
chown -R opendkim:opendkim /etc/opendkim
chmod 600 /etc/opendkim/keys/example.com/mail.private

# Get DNS record
cat /etc/opendkim/keys/example.com/mail.txt
```

Add to DNS:
```
mail._domainkey.example.com. IN TXT "v=DKIM1; k=rsa; p=MIGfMA0..."
```

### 2. Fail2ban

```bash
# Install fail2ban
apt install -y fail2ban

# Create jail for Postfix
cat > /etc/fail2ban/jail.d/postfix.conf << EOF
[postfix]
enabled = true
port = smtp,submission,smtps
filter = postfix
logpath = /var/log/mail.log
maxretry = 5
bantime = 3600

[dovecot]
enabled = true
port = imap,imaps,pop3,pop3s
filter = dovecot
logpath = /var/log/mail.log
maxretry = 5
bantime = 3600
EOF

# Restart fail2ban
systemctl restart fail2ban

# Check status
fail2ban-client status
```

### 3. Rate Limiting

Add to `/etc/postfix/main.cf`:

```
# Rate limiting
smtpd_client_connection_rate_limit = 10
smtpd_client_message_rate_limit = 20
smtpd_client_recipient_rate_limit = 50
smtpd_client_connection_count_limit = 50
```

### 4. Regular Backups

```bash
# Create backup script
cat > /usr/local/bin/backup-mail.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/backup/mail"
DATE=$(date +%Y%m%d)

mkdir -p $BACKUP_DIR

# Backup database
mysqldump -u mailuser -p'PASSWORD' mailserver | \
  gzip > $BACKUP_DIR/mailserver-$DATE.sql.gz

# Backup mail data
tar czf $BACKUP_DIR/vhosts-$DATE.tar.gz /var/mail/vhosts

# Backup configs
tar czf $BACKUP_DIR/configs-$DATE.tar.gz \
  /etc/postfix /etc/dovecot /etc/letsencrypt

# Remove old backups (keep 30 days)
find $BACKUP_DIR -mtime +30 -delete
EOF

chmod +x /usr/local/bin/backup-mail.sh

# Add to cron
echo "0 2 * * * /usr/local/bin/backup-mail.sh" | crontab -
```

---

## Testing

### Mail Client Configuration

#### Thunderbird / Outlook

```
Incoming Mail (IMAP):
  Server: mail.example.com
  Port: 993
  Security: SSL/TLS
  Authentication: Normal password
  Username: admin@example.com

Outgoing Mail (SMTP):
  Server: mail.example.com
  Port: 587
  Security: STARTTLS
  Authentication: Normal password
  Username: admin@example.com
```

#### Manual Testing Tools

```bash
# Test SMTP with authentication
openssl s_client -starttls smtp -connect localhost:587

# Commands:
EHLO localhost
AUTH LOGIN
# Enter base64 encoded username
# Enter base64 encoded password
MAIL FROM:<admin@example.com>
RCPT TO:<admin@example.com>
DATA
Subject: Test
Test message
.
QUIT

# Generate base64 for auth:
echo -n 'admin@example.com' | base64
echo -n 'password' | base64
```

### Monitoring Logs

```bash
# Real-time mail log
tail -f /var/log/mail.log

# Show only errors
tail -f /var/log/mail.err

# Postfix queue
postqueue -p

# Dovecot connections
doveadm who

# Mail statistics
pflogsumm /var/log/mail.log
```

---

## Troubleshooting

### Common Issues

#### 1. Cannot Connect to Port 25/587/993

**Symptom:**
```bash
telnet mail.example.com 25
# Connection refused or timeout
```

**Solutions:**

```bash
# Check if service is running
systemctl status postfix dovecot

# Check if ports are listening
ss -tlnp | grep -E '25|587|993'

# Check firewall
ufw status
iptables -L -n | grep -E '25|587|993'

# Check SELinux (if applicable)
getenforce
```

#### 2. Authentication Failures

**Symptom:**
```
Authentication failed: Invalid credentials
```

**Debug:**

```bash
# Test password hash
doveadm pw -s SHA512-CRYPT -p 'YourPassword'

# Test authentication
doveadm auth test user@example.com 'password'

# Check logs
tail -100 /var/log/mail.log | grep auth

# Verify user in database
mysql -u mailuser -p mailserver -e \
  "SELECT email FROM virtual_users WHERE email='user@example.com';"
```

#### 3. Mail Not Delivered (Stuck in Queue)

**Symptom:**
```bash
postqueue -p
# Shows messages stuck
```

**Solutions:**

```bash
# View queue details
postqueue -p

# View specific message
postcat -q QUEUE_ID

# Flush queue
postqueue -f

# Check mail log
tail -100 /var/log/mail.log

# Common issues:
# - Incorrect virtual_mailbox_maps
# - Permission issues on /var/mail/vhosts
# - Dovecot LMTP not running
```

#### 4. SSL Certificate Errors

**Symptom:**
```
SSL certificate verification failed
```

**Solutions:**

```bash
# Check certificate validity
openssl s_client -connect mail.example.com:993 -showcerts

# Check certificate files
ls -la /etc/letsencrypt/live/mail.example.com/

# Renew if needed
certbot renew --force-renewal

# Reload services
systemctl reload postfix dovecot
```

#### 5. Mail Marked as Spam

**Solutions:**

```bash
# Check SPF record
dig example.com TXT | grep spf

# Check DKIM record
dig mail._domainkey.example.com TXT

# Check reverse DNS
dig -x 203.0.113.10

# Test email reputation
# Use tools like:
# - https://mxtoolbox.com
# - https://www.mail-tester.com
```

### Diagnostic Commands

```bash
# Full Postfix configuration
postconf -n

# Full Dovecot configuration
doveconf -n

# Test MySQL lookups
postmap -q "domain.com" mysql:/etc/postfix/mysql-virtual-domains.cf
postmap -q "user@domain.com" mysql:/etc/postfix/mysql-virtual-users.cf

# Check mail queue
mailq

# View message content
postcat -q QUEUE_ID

# Test connection to MySQL
mysql -u mailuser -p -h localhost mailserver -e "SELECT COUNT(*) FROM virtual_users;"

# Check disk space
df -h /var/mail

# Check permissions
ls -la /var/mail/vhosts/
```

---

## Maintenance

### Daily Tasks

```bash
# Check mail queue
mailq

# Check disk space
df -h /var/mail

# Review logs for errors
grep -i error /var/log/mail.log | tail -20
```

### Weekly Tasks

```bash
# Review fail2ban bans
fail2ban-client status postfix
fail2ban-client status dovecot

# Check for updates
apt update
apt list --upgradable | grep -E 'postfix|dovecot|mysql'

# Review mail statistics
pflogsumm /var/log/mail.log.1
```

### Monthly Tasks

```bash
# Update system
apt update && apt upgrade -y

# Clean old mail logs
find /var/log -name "mail.log.*" -mtime +90 -delete

# Verify backups
ls -lh /backup/mail/

# Test certificate renewal
certbot renew --dry-run
```

### Performance Tuning

```bash
# Monitor resource usage
htop

# Check Postfix performance
postfix status

# Check Dovecot performance
doveadm stats dump

# Optimize MySQL
mysqltuner
```

---

## Support Resources

### Official Documentation

- **Postfix**: http://www.postfix.org/documentation.html
- **Dovecot**: https://doc.dovecot.org/
- **MySQL**: https://dev.mysql.com/doc/

### Useful Tools

- **MXToolbox**: https://mxtoolbox.com/ - DNS and mail server testing
- **Mail Tester**: https://www.mail-tester.com/ - Spam score testing
- **SSL Labs**: https://www.ssllabs.com/ssltest/ - SSL configuration testing

### Log Locations

```
/var/log/mail.log       - Main mail log
/var/log/mail.err       - Mail errors
/var/log/syslog         - System log
/var/log/auth.log       - Authentication log
/var/log/mysql/         - MySQL logs
```

---

## Appendix

### Complete Configuration File Reference

See individual files in:
- `postfix/` - All Postfix configuration files
- `dovecot/` - All Dovecot configuration files
- `database/` - Database schema and examples

### Environment Variables

```bash
# Used in installation script
HOSTNAME=mail.example.com
DOMAIN=example.com
DB_PASSWORD=secure_random_password
SSL_EMAIL=admin@example.com
```

### Default Ports Summary

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| SMTP | 25 | TCP | Mail transfer |
| Submission | 587 | TCP | Authenticated sending |
| SMTPS | 465 | TCP | Secure SMTP |
| IMAP | 143 | TCP | Email access |
| IMAPS | 993 | TCP | Secure IMAP |
| HTTP | 80 | TCP | Web & certbot |
| HTTPS | 443 | TCP | Secure web |

---

**Last Updated**: 2026-02-27
**Version**: 1.0.0
