# Mail Server Deployment Configuration

This directory contains all configuration files and scripts needed to deploy the mail system on a new server.

## Prerequisites

- Debian 11/12 or Ubuntu 20.04/22.04 LTS
- Root access
- Domain name with proper DNS records
- MySQL/MariaDB server

## Quick Start

1. Clone this repository
2. Update configuration files with your domain and credentials
3. Run the installation script
4. Configure DNS records
5. Obtain SSL certificates
6. Test mail delivery

## Directory Structure

```
deployment/
├── postfix/              # Postfix mail server configuration
│   ├── main.cf          # Main Postfix configuration
│   ├── master.cf        # Postfix services configuration
│   ├── mysql-virtual-domains.cf
│   ├── mysql-virtual-users.cf
│   ├── mysql-virtual-aliases.cf
│   └── mysql-sender-login-maps.cf
├── dovecot/             # Dovecot IMAP/LMTP configuration
│   ├── dovecot.conf     # Main Dovecot configuration
│   ├── dovecot-sql.conf.ext
│   ├── 10-auth.conf     # Authentication settings
│   ├── 10-mail.conf     # Mail location and namespaces
│   ├── 10-master.conf   # Service configuration
│   ├── 10-ssl.conf      # SSL/TLS settings
│   ├── 90-plugin.conf   # Plugin configuration
│   └── auth-sql.conf.ext
├── database/            # Database schema and scripts
│   └── schema.sql       # Complete database schema
├── scripts/             # Installation and setup scripts
│   └── install.sh       # Automated installation script
└── README.md           # This file
```

## Installation Steps

### 1. Update System and Install Packages

```bash
apt update && apt upgrade -y
apt install -y postfix postfix-mysql dovecot-core dovecot-imapd \
  dovecot-lmtpd dovecot-mysql mariadb-server certbot
```

### 2. Configure MySQL Database

```bash
# Secure MySQL installation
mysql_secure_installation

# Create database and user
mysql -u root -p << EOF
CREATE DATABASE mailserver;
CREATE USER 'mailuser'@'localhost' IDENTIFIED BY 'YOUR_SECURE_PASSWORD';
GRANT ALL PRIVILEGES ON mailserver.* TO 'mailuser'@'localhost';
FLUSH PRIVILEGES;
EOF

# Import schema
mysql -u root -p mailserver < deployment/database/schema.sql
```

### 3. Configure Postfix

```bash
# Update configuration files with your details
# Replace CHANGE_ME_HOSTNAME with your domain
# Replace CHANGE_ME_DB_PASSWORD with your database password

# Copy Postfix configuration files
cp deployment/postfix/main.cf /etc/postfix/main.cf
cp deployment/postfix/master.cf /etc/postfix/master.cf
cp deployment/postfix/mysql-*.cf /etc/postfix/

# Set proper permissions
chmod 640 /etc/postfix/mysql-*.cf
chown root:postfix /etc/postfix/mysql-*.cf

# Update /etc/mailname
echo "yourdomain.com" > /etc/mailname

# Restart Postfix
systemctl restart postfix
```

### 4. Configure Dovecot

```bash
# Backup original configuration
mv /etc/dovecot/dovecot.conf /etc/dovecot/dovecot.conf.orig

# Copy configuration files
cp deployment/dovecot/dovecot.conf /etc/dovecot/
cp deployment/dovecot/dovecot-sql.conf.ext /etc/dovecot/
cp deployment/dovecot/auth-sql.conf.ext /etc/dovecot/conf.d/
cp deployment/dovecot/10-*.conf /etc/dovecot/conf.d/
cp deployment/dovecot/90-*.conf /etc/dovecot/conf.d/

# Set proper permissions
chmod 600 /etc/dovecot/dovecot-sql.conf.ext
chown root:root /etc/dovecot/dovecot-sql.conf.ext

# Restart Dovecot
systemctl restart dovecot
```

### 5. Create Mail Directory Structure

```bash
# Create virtual mail user
groupadd -g 5000 vmail
useradd -g vmail -u 5000 vmail -d /var/mail

# Create mail directory
mkdir -p /var/mail/vhosts
chown -R vmail:vmail /var/mail/vhosts
chmod -R 770 /var/mail/vhosts
```

### 6. Configure DNS Records

Add the following DNS records for your domain:

```
# MX Record
yourdomain.com.    IN MX 10 mail.yourdomain.com.

# A Record for mail server
mail.yourdomain.com.    IN A    YOUR_SERVER_IP

# SPF Record
yourdomain.com.    IN TXT "v=spf1 mx ~all"

# DMARC Record (optional but recommended)
_dmarc.yourdomain.com.    IN TXT "v=DMARC1; p=none; rua=mailto:postmaster@yourdomain.com"
```

### 7. Obtain SSL Certificates

```bash
# Stop services temporarily
systemctl stop postfix dovecot

# Obtain certificate
certbot certonly --standalone -d mail.yourdomain.com

# Restart services
systemctl start postfix dovecot

# Setup auto-renewal
certbot renew --dry-run
```

### 8. Deploy Web Applications

```bash
# Build and deploy mailadmin
cd mailadmin/backend
go build -o mailadmin
cp mailadmin /usr/local/bin/

cd ../frontend
npm install
npm run build

# Build and deploy groupware
cd ../../groupware
go build -o groupware
cp groupware /usr/local/bin/

cd frontend
npm install
npm run build

# Build and deploy webmail
cd ../../webmail/backend
go build -o webmail
cp webmail /usr/local/bin/

cd ../frontend
npm install
npm run build
```

### 9. Create Systemd Services

Create service files for each application in `/etc/systemd/system/`:

**mailadmin.service:**
```ini
[Unit]
Description=Mail Admin Service
After=network.target mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/mailadmin/backend
ExecStart=/usr/local/bin/mailadmin
Restart=always

[Install]
WantedBy=multi-user.target
```

Similar services for groupware and webmail.

### 10. Enable and Start Services

```bash
systemctl enable postfix dovecot mailadmin groupware webmail
systemctl start mailadmin groupware webmail
```

## Configuration Variables

Update these values in the configuration files:

- `CHANGE_ME_HOSTNAME`: Your mail server hostname (e.g., mail.yourdomain.com)
- `CHANGE_ME_DB_PASSWORD`: MySQL mailuser password

## Testing

### Test SMTP

```bash
telnet localhost 25
EHLO localhost
QUIT
```

### Test IMAP

```bash
telnet localhost 143
```

### Test Authentication

```bash
doveadm auth test user@yourdomain.com password
```

### Send Test Email

```bash
echo "Test email body" | mail -s "Test Subject" user@yourdomain.com
```

## Firewall Configuration

Open required ports:

```bash
ufw allow 25/tcp   # SMTP
ufw allow 587/tcp  # Submission
ufw allow 465/tcp  # SMTPS
ufw allow 143/tcp  # IMAP
ufw allow 993/tcp  # IMAPS
ufw allow 80/tcp   # HTTP (for web apps)
ufw allow 443/tcp  # HTTPS
```

## Security Recommendations

1. Enable fail2ban for Postfix and Dovecot
2. Configure DKIM signing (opendkim)
3. Enable SpamAssassin
4. Regular security updates
5. Monitor logs regularly
6. Use strong passwords for all accounts
7. Enable firewall (ufw/iptables)
8. Keep SSL certificates up to date

## Troubleshooting

### Check Service Status

```bash
systemctl status postfix
systemctl status dovecot
```

### View Logs

```bash
tail -f /var/log/mail.log
tail -f /var/log/mail.err
journalctl -u postfix -f
journalctl -u dovecot -f
```

### Test Configuration

```bash
postfix check
doveconf -n
```

### Common Issues

1. **Cannot connect to MySQL**: Check credentials in mysql-*.cf files
2. **SSL certificate errors**: Verify certificate paths in configuration
3. **Permission denied**: Check ownership of /var/mail/vhosts
4. **Authentication failures**: Verify dovecot-sql.conf.ext configuration

## Support

For issues and questions:
- Check logs first
- Review configuration files
- Consult Postfix/Dovecot documentation

## License

[Add your license here]
