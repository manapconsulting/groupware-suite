#!/bin/bash

# Mail Server Installation Script
# This script automates the installation of Postfix, Dovecot, and related components

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -ne 0 ]; then
   echo -e "${RED}Please run as root${NC}"
   exit 1
fi

echo "=================================="
echo "  Mail Server Installation"
echo "=================================="
echo ""

# Prompt for configuration
read -p "Enter your mail server hostname (e.g., mail.example.com): " HOSTNAME
read -p "Enter your primary domain (e.g., example.com): " DOMAIN
read -p "Enter MySQL root password: " -s MYSQL_ROOT_PASS
echo ""
read -p "Enter password for mailuser database user: " -s DB_PASSWORD
echo ""
read -p "Enter your email address for SSL certificates: " SSL_EMAIL
echo ""

echo -e "${YELLOW}Installing required packages...${NC}"
apt update
apt install -y postfix postfix-mysql dovecot-core dovecot-imapd \
  dovecot-lmtpd dovecot-mysql mariadb-server certbot \
  build-essential golang-go nodejs npm

echo -e "${GREEN}Packages installed successfully${NC}"

echo -e "${YELLOW}Configuring MySQL...${NC}"
mysql -u root -p"${MYSQL_ROOT_PASS}" << EOF
CREATE DATABASE IF NOT EXISTS mailserver;
CREATE USER IF NOT EXISTS 'mailuser'@'localhost' IDENTIFIED BY '${DB_PASSWORD}';
GRANT ALL PRIVILEGES ON mailserver.* TO 'mailuser'@'localhost';
FLUSH PRIVILEGES;
EOF

# Import schema
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
DEPLOY_DIR="$(dirname "$SCRIPT_DIR")"

mysql -u root -p"${MYSQL_ROOT_PASS}" mailserver < "${DEPLOY_DIR}/database/schema.sql"

echo -e "${GREEN}Database configured successfully${NC}"

echo -e "${YELLOW}Creating virtual mail user...${NC}"
groupadd -g 5000 vmail 2>/dev/null || true
useradd -g vmail -u 5000 vmail -d /var/mail 2>/dev/null || true

mkdir -p /var/mail/vhosts
chown -R vmail:vmail /var/mail/vhosts
chmod -R 770 /var/mail/vhosts

echo -e "${GREEN}Virtual mail user created${NC}"

echo -e "${YELLOW}Configuring Postfix...${NC}"
# Backup original configs
cp /etc/postfix/main.cf /etc/postfix/main.cf.backup 2>/dev/null || true
cp /etc/postfix/master.cf /etc/postfix/master.cf.backup 2>/dev/null || true

# Copy and update configuration files
cp "${DEPLOY_DIR}/postfix/main.cf" /etc/postfix/main.cf
cp "${DEPLOY_DIR}/postfix/master.cf" /etc/postfix/master.cf
cp "${DEPLOY_DIR}/postfix/mysql-"*.cf /etc/postfix/

# Update hostname and password
sed -i "s/CHANGE_ME_HOSTNAME/${HOSTNAME}/g" /etc/postfix/main.cf
sed -i "s/CHANGE_ME_DB_PASSWORD/${DB_PASSWORD}/g" /etc/postfix/mysql-*.cf

# Update /etc/mailname
echo "${DOMAIN}" > /etc/mailname

# Set permissions
chmod 640 /etc/postfix/mysql-*.cf
chown root:postfix /etc/postfix/mysql-*.cf

echo -e "${GREEN}Postfix configured${NC}"

echo -e "${YELLOW}Configuring Dovecot...${NC}"
# Backup original config
cp /etc/dovecot/dovecot.conf /etc/dovecot/dovecot.conf.backup 2>/dev/null || true

# Copy configuration files
cp "${DEPLOY_DIR}/dovecot/dovecot.conf" /etc/dovecot/
cp "${DEPLOY_DIR}/dovecot/dovecot-sql.conf.ext" /etc/dovecot/
cp "${DEPLOY_DIR}/dovecot/auth-sql.conf.ext" /etc/dovecot/conf.d/
cp "${DEPLOY_DIR}/dovecot/10-"*.conf /etc/dovecot/conf.d/
cp "${DEPLOY_DIR}/dovecot/90-"*.conf /etc/dovecot/conf.d/

# Update hostname and password
sed -i "s/CHANGE_ME_HOSTNAME/${HOSTNAME}/g" /etc/dovecot/conf.d/10-ssl.conf
sed -i "s/CHANGE_ME_DB_PASSWORD/${DB_PASSWORD}/g" /etc/dovecot/dovecot-sql.conf.ext

# Set permissions
chmod 600 /etc/dovecot/dovecot-sql.conf.ext
chown root:root /etc/dovecot/dovecot-sql.conf.ext

echo -e "${GREEN}Dovecot configured${NC}"

echo -e "${YELLOW}Obtaining SSL certificates...${NC}"
# Stop services temporarily
systemctl stop postfix dovecot 2>/dev/null || true

# Obtain certificate
certbot certonly --standalone -d "${HOSTNAME}" --email "${SSL_EMAIL}" --agree-tos --non-interactive

echo -e "${GREEN}SSL certificates obtained${NC}"

echo -e "${YELLOW}Starting services...${NC}"
systemctl enable postfix dovecot
systemctl start postfix dovecot

echo -e "${GREEN}Services started${NC}"

echo ""
echo "=================================="
echo "  Installation Complete!"
echo "=================================="
echo ""
echo "Next steps:"
echo "1. Configure DNS records for your domain:"
echo "   MX record: ${DOMAIN} -> ${HOSTNAME}"
echo "   A record: ${HOSTNAME} -> [Your Server IP]"
echo ""
echo "2. Add your first domain to the database:"
echo "   mysql -u mailuser -p mailserver"
echo "   INSERT INTO virtual_domains (name) VALUES ('${DOMAIN}');"
echo ""
echo "3. Add your first user (replace PASSWORD_HASH with actual hash):"
echo "   INSERT INTO virtual_users (domain_id, email, password)"
echo "   VALUES (1, 'user@${DOMAIN}', 'PASSWORD_HASH');"
echo ""
echo "4. Generate password hash with:"
echo "   doveadm pw -s SHA512-CRYPT"
echo ""
echo "5. Test your configuration:"
echo "   postfix check"
echo "   doveconf -n"
echo ""
echo "6. Monitor logs:"
echo "   tail -f /var/log/mail.log"
echo ""

exit 0
