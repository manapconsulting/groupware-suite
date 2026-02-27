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

## Prerequisites

- Go 1.x or higher
- Node.js and npm
- MySQL database

## Setup

Each application has its own setup requirements. Please refer to individual application directories for specific installation instructions.

## License

[Add your license here]
