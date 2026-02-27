# Mail System Monorepo

Complete mail server solution with web-based administration, groupware (CalDAV/CardDAV), and webmail client.

## System Overview

```
┌──────────────────────────────────────────────────────────────┐
│                     MAIL SYSTEM STACK                         │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐        │
│  │ Mail Admin  │  │  Groupware   │  │  Webmail    │        │
│  │             │  │              │  │             │        │
│  │ - Users     │  │ - CalDAV     │  │ - IMAP UI   │        │
│  │ - Domains   │  │ - CardDAV    │  │ - Compose   │        │
│  │ - Aliases   │  │ - IMAP/SMTP  │  │ - Folders   │        │
│  └─────┬───────┘  └──────┬───────┘  └──────┬──────┘        │
│        │                 │                  │                │
│        └─────────────────┼──────────────────┘                │
│                          │                                   │
│  ┌───────────────────────▼────────────────────────┐         │
│  │           MySQL Database (mailserver)          │         │
│  │  - virtual_domains    - calendars              │         │
│  │  - virtual_users      - events                 │         │
│  │  - virtual_aliases    - contacts               │         │
│  └────────────────────────────────────────────────┘         │
│                                                               │
├──────────────────────────────────────────────────────────────┤
│                  MAIL SERVER BACKEND                          │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐              ┌──────────────┐             │
│  │   POSTFIX    │  ◄─ LMTP ─►  │   DOVECOT    │             │
│  │   (MTA)      │              │   (IMAP)     │             │
│  │              │              │              │             │
│  │ • SMTP       │              │ • Maildir    │             │
│  │ • Auth       │              │ • ACL        │             │
│  │ • Virtual    │              │ • SSL/TLS    │             │
│  └──────────────┘              └──────────────┘             │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

This monorepo contains three interconnected mail system applications.

## Applications

### 1. Mail Admin (`/mailadmin`)
Mail server administration interface for managing users, domains, and mail system configuration.

**Tech Stack:**
- Backend: Go
- Frontend: TypeScript + Vite
- Location: `/opt/mailadmin`

**Structure:**
- `backend/` - Go backend API
- `frontend/` - React/TypeScript frontend

### 2. Groupware (`/groupware`)
Comprehensive groupware solution with email, calendar (CalDAV), and contacts (CardDAV) functionality.

**Tech Stack:**
- Backend: Go
- Frontend: TypeScript + Vite
- Protocols: IMAP, SMTP, CalDAV, CardDAV
- Location: `/opt/groupware`

**Features:**
- Email management (IMAP/SMTP)
- Calendar synchronization (CalDAV)
- Contact management (CardDAV)
- Web interface

### 3. Webmail (`/webmail`)
Web-based email client interface.

**Tech Stack:**
- Backend: Go
- Frontend: TypeScript + Vite
- Location: `/opt/webmail`

**Structure:**
- `backend/` - Go backend API
- `frontend/` - React/TypeScript frontend

## Repository Structure

```
groupware-suite/
├── deployment/               # Complete mail server deployment
│   ├── postfix/             # Postfix configuration templates
│   ├── dovecot/             # Dovecot configuration templates
│   ├── database/            # MySQL schema
│   ├── scripts/             # Installation automation
│   └── README.md            # Detailed deployment guide
│
├── mailadmin/               # Web-based mail administration
│   ├── backend/             # Go API server
│   └── frontend/            # React/TypeScript UI
│
├── groupware/               # CalDAV/CardDAV/IMAP groupware
│   ├── backend/             # Go server (CalDAV/CardDAV)
│   └── frontend/            # Web interface
│
├── webmail/                 # Webmail client
│   ├── backend/             # Go IMAP/SMTP proxy
│   └── frontend/            # Email client UI
│
└── run-tests.sh             # Test runner for all apps
```

## Development

### Running Tests

Use the provided test runner script:

```bash
# Run all tests
./run-tests.sh all

# Run specific application tests
./run-tests.sh mailadmin
./run-tests.sh groupware

# Run benchmarks
./run-tests.sh bench
```

### Environment Variables

The test runner uses the following environment variables:
- `DATABASE_URL` - MySQL connection string
- `JWT_SECRET` - Secret key for JWT tokens
- `ADMIN_USERS` - Admin user credentials

## Deployment

### Full Server Setup

The `/deployment` directory contains complete configuration for deploying this mail system on a new server, including:

- **Postfix** - Mail Transfer Agent (MTA) configuration
- **Dovecot** - IMAP/LMTP server configuration
- **MySQL Schema** - Database structure for all applications
- **Automated Installation Script** - One-command setup

**Quick Start:**

```bash
cd deployment
# Review and edit configuration files first!
sudo ./scripts/install.sh
```

For detailed deployment instructions, see [deployment/README.md](deployment/README.md)

### What's Included in Deployment

- Virtual domain and user management
- MySQL-based authentication
- SSL/TLS with Let's Encrypt support
- SASL authentication
- Shared mailbox support (ACL)
- CalDAV/CardDAV database schema
- Multi-domain SSL certificates (SNI)

## Prerequisites

- Debian 11/12 or Ubuntu 20.04/22.04 LTS
- Go 1.x or higher
- Node.js and npm
- MySQL/MariaDB database
- Root access (for deployment)
- Domain name with DNS access

## Setup

### For Development

Each application has its own setup requirements. Please refer to individual application directories for specific installation instructions.

### For Production

1. Clone this repository to your server
2. Review and customize files in `/deployment` directory
3. Run the installation script: `cd deployment && sudo ./scripts/install.sh`
4. Configure DNS records (MX, A, SPF, DMARC)
5. Build and deploy the web applications
6. Create systemd services for each app

See [deployment/README.md](deployment/README.md) for complete production setup guide.

## Quick Reference

### Mail Server Components

| Component | Purpose | Ports |
|-----------|---------|-------|
| Postfix | SMTP server (send/receive) | 25, 587, 465 |
| Dovecot | IMAP server (read mail) | 143, 993 |
| MySQL | User/domain database | 3306 (local) |
| Mail Admin | Web-based administration | 8080 |
| Groupware | CalDAV/CardDAV/Email | 8081 |
| Webmail | Email web client | 8082 |

### Key Features

```
┌─────────────────────────────────────────────────────────┐
│  FEATURES                                                │
├─────────────────────────────────────────────────────────┤
│  ✓ Virtual domain management                            │
│  ✓ Multiple domains on one server                       │
│  ✓ Web-based user/domain administration                 │
│  ✓ Email aliases and forwarding                         │
│  ✓ CalDAV calendar synchronization                      │
│  ✓ CardDAV contact management                           │
│  ✓ Shared mailboxes (ACL)                               │
│  ✓ SSL/TLS encryption (Let's Encrypt)                   │
│  ✓ SMTP authentication (SASL)                           │
│  ✓ Sender permission controls                           │
│  ✓ MySQL-based authentication                           │
│  ✓ Multi-domain SSL certificates (SNI)                  │
└─────────────────────────────────────────────────────────┘
```

### Common Tasks

#### Adding a New Domain

```bash
# 1. Add DNS records (MX, A, SPF, DMARC)
# 2. Add to database
mysql -u mailuser -p mailserver << EOF
INSERT INTO virtual_domains (name) VALUES ('newdomain.com');
EOF

# 3. Get SSL certificate
certbot certonly --standalone -d mail.newdomain.com

# 4. Update Dovecot SNI (if needed)
# 5. Create users via Mail Admin web interface
```

#### Creating a User

```bash
# Generate password hash
doveadm pw -s SHA512-CRYPT

# Add to database
mysql -u mailuser -p mailserver << EOF
INSERT INTO virtual_users (domain_id, email, password)
VALUES (
  (SELECT id FROM virtual_domains WHERE name='domain.com'),
  'user@domain.com',
  'PASSWORD_HASH'
);
EOF
```

#### Checking Mail Logs

```bash
# Real-time monitoring
tail -f /var/log/mail.log

# Check for errors
grep -i error /var/log/mail.log | tail -20

# View mail queue
mailq

# Check service status
systemctl status postfix dovecot
```

### Troubleshooting Quick Guide

| Issue | Quick Check |
|-------|-------------|
| Can't connect to server | `ss -tlnp \| grep -E '25\|587\|993'` |
| Authentication fails | `doveadm auth test user@domain.com password` |
| Mail not delivered | `tail -f /var/log/mail.log` then send test |
| SSL errors | `openssl s_client -connect mail.domain.com:993` |
| Queue stuck | `postqueue -p` and `postcat -q QUEUE_ID` |

### Architecture Diagram

```
                    ┌───────────────┐
                    │   INTERNET    │
                    └───────┬───────┘
                            │
            ┌───────────────┼───────────────┐
            │               │               │
         Port 25         Port 587        Port 993
      (Incoming)      (Sending Auth)    (IMAP Read)
            │               │               │
            ▼               ▼               │
      ┌─────────────────────────┐          │
      │       POSTFIX           │          │
      │  - Receive mail         │          │
      │  - Authenticate users   │◄─SASL───┐│
      │  - Check permissions    │         ││
      └──────────┬──────────────┘         ││
                 │ LMTP                   ││
                 ▼                        ││
      ┌─────────────────────────┐        ││
      │       DOVECOT           │────────┘│
      │  - Store mail           │         │
      │  - IMAP access          │◄────────┘
      │  - SASL auth provider   │
      └──────────┬──────────────┘
                 │
                 ▼
      ┌─────────────────────────┐
      │  /var/mail/vhosts/      │
      │    domain.com/          │
      │      user@domain.com/   │
      │        Maildir/         │
      └─────────────────────────┘
                 ▲
                 │
      ┌──────────┴──────────────┐
      │      MySQL DB           │
      │  - Users                │
      │  - Domains              │
      │  - Aliases              │
      │  - Calendars/Contacts   │
      └─────────────────────────┘
```

## Documentation

- **[Deployment Guide](deployment/README.md)** - Complete server setup with diagrams and examples
- **[Mail Admin](mailadmin/README.md)** - Web administration interface
- **[Groupware](groupware/README.md)** - CalDAV/CardDAV documentation
- **[Webmail](webmail/README.md)** - Webmail client setup

## Support & Resources

- **Repository**: https://github.com/manapconsulting/groupware-suite
- **Issues**: Report bugs and request features via GitHub Issues
- **Testing Tools**:
  - [MXToolbox](https://mxtoolbox.com/) - DNS and mail server testing
  - [Mail Tester](https://www.mail-tester.com/) - Spam score testing
  - [SSL Labs](https://www.ssllabs.com/ssltest/) - SSL configuration testing

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

- Read the [Code of Conduct](CODE_OF_CONDUCT.md)
- Check out [open issues](https://github.com/manapconsulting/groupware-suite/issues)
- Submit [pull requests](https://github.com/manapconsulting/groupware-suite/pulls)

## Security

For security concerns, please review our [Security Policy](SECURITY.md). To report vulnerabilities privately, email security@manapconsulting.com.

## License

This project is licensed under the **GNU General Public License v3.0** - see the [LICENSE](LICENSE) file for details.

**Copyright (C) 2024-2026 Manap Consulting**

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.
