# Mail System Monorepo

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

## License

[Add your license here]
