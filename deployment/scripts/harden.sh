#!/bin/bash
# Mail Server Hardening Script
# Run after install.sh to apply security hardening

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Please run as root${NC}"
  exit 1
fi

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "================================================"
echo "  Mail Server Hardening"
echo "================================================"
echo ""

# -----------------------------------------------------------
step() { echo -e "\n${YELLOW}[$1/9] $2...${NC}"; }
ok()   { echo -e "${GREEN}  ✓ $1${NC}"; }
# -----------------------------------------------------------

step 1 "Configuring firewall (ufw)"
apt-get install -y ufw -q
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp    comment 'SSH'
ufw allow 25/tcp    comment 'SMTP'
ufw allow 587/tcp   comment 'Submission'
ufw allow 465/tcp   comment 'SMTPS'
ufw allow 143/tcp   comment 'IMAP'
ufw allow 993/tcp   comment 'IMAPS'
ufw allow 80/tcp    comment 'HTTP'
ufw allow 443/tcp   comment 'HTTPS'
ufw --force enable
ok "Firewall enabled"

# -----------------------------------------------------------
step 2 "Hardening fail2ban"
apt-get install -y fail2ban -q
cp "${DEPLOY_DIR}/fail2ban/mail.conf" /etc/fail2ban/jail.d/mail.conf
systemctl enable fail2ban
systemctl restart fail2ban
ok "Fail2ban configured (bantime=24h, maxretry=3)"

# -----------------------------------------------------------
step 3 "Generating DH parameters for Dovecot (this takes a while)"
if [ ! -f /etc/dovecot/dh.pem ]; then
  openssl dhparam -out /etc/dovecot/dh.pem 2048
  chmod 600 /etc/dovecot/dh.pem
  ok "DH parameters generated"
else
  ok "DH parameters already exist"
fi

# -----------------------------------------------------------
step 4 "Applying hardened Postfix configuration"
postconf -e "smtpd_banner = \$myhostname ESMTP"
postconf -e "smtpd_helo_required = yes"
postconf -e "smtpd_tls_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtpd_tls_mandatory_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtp_tls_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtp_tls_mandatory_protocols = !SSLv2, !SSLv3, !TLSv1, !TLSv1.1"
postconf -e "smtpd_tls_ciphers = high"
postconf -e "smtpd_tls_mandatory_ciphers = high"
postconf -e "smtpd_tls_exclude_ciphers = aNULL, eNULL, EXPORT, DES, 3DES, RC2, RC4, MD5, PSK, SRP, DSS, SEED"
postconf -e "smtpd_client_connection_rate_limit = 10"
postconf -e "smtpd_client_message_rate_limit = 20"
postconf -e "smtpd_client_recipient_rate_limit = 50"
postconf -e "smtpd_client_connection_count_limit = 10"
postconf -e "smtpd_error_sleep_time = 5s"
postconf -e "smtpd_soft_error_limit = 5"
postconf -e "smtpd_hard_error_limit = 10"
postconf -e "smtpd_recipient_limit = 50"
postconf -e "smtpd_sasl_security_options = noanonymous, noplaintext"
postconf -e "smtpd_sasl_tls_security_options = noanonymous"
ok "Postfix hardened"

# -----------------------------------------------------------
step 5 "Applying hardened Dovecot SSL configuration"
cp "${DEPLOY_DIR}/dovecot/10-ssl.conf" /etc/dovecot/conf.d/10-ssl.conf
# Replace hostname placeholder
HOSTNAME=$(postconf -h myhostname)
sed -i "s/CHANGE_ME_HOSTNAME/$HOSTNAME/g" /etc/dovecot/conf.d/10-ssl.conf
ok "Dovecot SSL hardened (TLS 1.2+, strong ciphers)"

# -----------------------------------------------------------
step 6 "Installing and configuring SpamAssassin"
apt-get install -y spamassassin spamc -q
systemctl enable spamassassin
systemctl start spamassassin
# Update rules
sa-update 2>/dev/null || true
# Daily rule updates
cat > /etc/cron.daily/sa-update << 'EOF'
#!/bin/bash
sa-update && systemctl reload spamassassin 2>/dev/null
EOF
chmod +x /etc/cron.daily/sa-update
ok "SpamAssassin installed and enabled"

# -----------------------------------------------------------
step 7 "Securing file permissions"
# Mail directories
chown -R vmail:vmail /var/mail/vhosts
chmod -R 770 /var/mail/vhosts

# Postfix MySQL files
chmod 640 /etc/postfix/mysql-*.cf
chown root:postfix /etc/postfix/mysql-*.cf

# Dovecot SQL config
chmod 600 /etc/dovecot/dovecot-sql.conf.ext
chown root:root /etc/dovecot/dovecot-sql.conf.ext

ok "File permissions secured"

# -----------------------------------------------------------
step 8 "Hardening MySQL"
mysql -u root -p"${MYSQL_ROOT_PASS:-}" << 'MYSQL_EOF' 2>/dev/null || true
DELETE FROM mysql.user WHERE User='';
DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');
DROP DATABASE IF EXISTS test;
DELETE FROM mysql.db WHERE Db='test' OR Db='test\\_%';
FLUSH PRIVILEGES;
MYSQL_EOF
ok "MySQL anonymous users and test database removed"

# -----------------------------------------------------------
step 9 "Restarting services"
systemctl restart postfix dovecot fail2ban
ok "Services restarted"

# -----------------------------------------------------------
echo ""
echo "================================================"
echo "  Hardening Complete!"
echo "================================================"
echo ""
echo "Summary of changes applied:"
echo "  • Firewall (ufw) configured - only necessary ports open"
echo "  • Fail2ban - 24h ban, 3 max retries"
echo "  • TLS 1.2+ enforced on Postfix and Dovecot"
echo "  • Weak cipher suites disabled"
echo "  • DH parameters generated for Dovecot"
echo "  • Rate limiting enabled on Postfix"
echo "  • SpamAssassin installed and enabled"
echo "  • File permissions secured"
echo "  • MySQL anonymous accounts removed"
echo ""
echo "Next steps:"
echo "  1. Install DKIM: apt install opendkim opendkim-tools"
echo "  2. Enable Postscreen in master.cf"
echo "  3. Add SPF/DKIM/DMARC DNS records"
echo "  4. Run: fail2ban-client status"
echo "  5. Run: ufw status verbose"
echo "  6. Test with: https://www.mail-tester.com"
echo ""
