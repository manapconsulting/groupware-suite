# Contributing to Mail System Groupware Suite

Thank you for your interest in contributing to our mail server solution! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Reporting Bugs](#reporting-bugs)
- [Suggesting Features](#suggesting-features)

## Code of Conduct

This project adheres to a Code of Conduct that all contributors are expected to follow. Please read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before contributing.

## Getting Started

### Prerequisites

- Go 1.19 or higher
- Node.js 16 or higher
- MySQL/MariaDB 10.5+
- Git
- Basic understanding of mail servers (Postfix, Dovecot)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/groupware-suite.git
cd groupware-suite
```

3. Add upstream remote:

```bash
git remote add upstream https://github.com/manapconsulting/groupware-suite.git
```

4. Create a new branch for your feature:

```bash
git checkout -b feature/your-feature-name
```

## Development Setup

### Backend (Go)

Each application has its own Go backend:

```bash
# Mail Admin
cd mailadmin/backend
go mod download
go build

# Groupware
cd groupware
go mod download
go build

# Webmail
cd webmail/backend
go mod download
go build
```

### Frontend (TypeScript/React)

```bash
# Mail Admin
cd mailadmin/frontend
npm install
npm run dev

# Groupware
cd groupware/frontend
npm install
npm run dev

# Webmail
cd webmail/frontend
npm install
npm run dev
```

### Database Setup

```bash
# Import schema
mysql -u root -p mailserver < deployment/database/schema.sql

# Run migrations (if any)
# ...
```

## How to Contribute

### Types of Contributions

We welcome various types of contributions:

- **Bug fixes** - Fix issues in the codebase
- **New features** - Add new functionality
- **Documentation** - Improve or add documentation
- **Tests** - Add or improve test coverage
- **Performance** - Optimize existing code
- **Security** - Fix security vulnerabilities
- **Translations** - Add language support

### Contribution Workflow

1. **Check existing issues** - See if your idea/bug is already reported
2. **Create an issue** - Discuss your proposal before coding
3. **Wait for feedback** - Get approval before starting major work
4. **Code your changes** - Follow our coding standards
5. **Test thoroughly** - Ensure all tests pass
6. **Submit a pull request** - Reference the related issue

## Coding Standards

### Go Code

Follow standard Go conventions:

```go
// Good
func CreateUser(email string, password string) (*User, error) {
    if email == "" {
        return nil, errors.New("email is required")
    }

    hashedPassword, err := hashPassword(password)
    if err != nil {
        return nil, fmt.Errorf("failed to hash password: %w", err)
    }

    user := &User{
        Email:    email,
        Password: hashedPassword,
    }

    return user, nil
}
```

**Guidelines:**
- Use `gofmt` and `golint`
- Handle all errors explicitly
- Use meaningful variable names
- Add comments for exported functions
- Keep functions small and focused
- Use early returns to reduce nesting

### TypeScript/React Code

Follow modern React best practices:

```typescript
// Good
interface UserFormProps {
  onSubmit: (email: string, password: string) => Promise<void>;
  loading?: boolean;
}

export const UserForm: React.FC<UserFormProps> = ({ onSubmit, loading = false }) => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await onSubmit(email, password);
  };

  return (
    <form onSubmit={handleSubmit}>
      {/* ... */}
    </form>
  );
};
```

**Guidelines:**
- Use TypeScript for type safety
- Use functional components with hooks
- Follow ESLint configuration
- Use meaningful component and prop names
- Keep components small and reusable
- Avoid inline styles (use CSS modules or styled-components)

### SQL/Database

```sql
-- Good: Clear, formatted, with comments
-- Add a new user to a domain
INSERT INTO virtual_users (domain_id, email, password)
VALUES (
  (SELECT id FROM virtual_domains WHERE name = ?),
  ?,
  ?
);

-- Bad: Unclear, no error handling
INSERT INTO virtual_users VALUES (1, 'user@domain.com', 'hash');
```

**Guidelines:**
- Use prepared statements (prevent SQL injection)
- Add comments for complex queries
- Use meaningful table and column names
- Add indexes for frequently queried fields
- Use transactions for multi-step operations

## Testing Guidelines

### Backend Tests (Go)

```go
func TestCreateUser(t *testing.T) {
    tests := []struct {
        name        string
        email       string
        password    string
        wantErr     bool
        errContains string
    }{
        {
            name:     "valid user",
            email:    "test@example.com",
            password: "SecurePass123",
            wantErr:  false,
        },
        {
            name:        "empty email",
            email:       "",
            password:    "SecurePass123",
            wantErr:     true,
            errContains: "email is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            user, err := CreateUser(tt.email, tt.password)

            if tt.wantErr {
                if err == nil {
                    t.Error("expected error, got nil")
                }
                if !strings.Contains(err.Error(), tt.errContains) {
                    t.Errorf("error = %v, want error containing %v", err, tt.errContains)
                }
                return
            }

            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }

            if user.Email != tt.email {
                t.Errorf("email = %v, want %v", user.Email, tt.email)
            }
        })
    }
}
```

### Running Tests

```bash
# Run all tests
./run-tests.sh all

# Run specific app tests
./run-tests.sh mailadmin
./run-tests.sh groupware

# Run with coverage
go test -cover ./...

# Run benchmarks
./run-tests.sh bench
```

### Test Requirements

- All new features must include tests
- Bug fixes should include regression tests
- Aim for >80% code coverage
- Test both success and error cases
- Use table-driven tests for multiple scenarios

## Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

### Examples

```
feat(mailadmin): add bulk user import functionality

Add ability to import multiple users from CSV file.
Includes validation and error reporting.

Closes #123
```

```
fix(groupware): resolve CalDAV sync issue with recurring events

Fixed bug where recurring events were not properly synchronized
when the recurrence rule contained EXDATE entries.

Fixes #456
```

```
docs(deployment): add DKIM configuration guide

Added step-by-step guide for configuring DKIM with OpenDKIM,
including DNS record examples and testing instructions.
```

### Commit Message Guidelines

- Use imperative mood ("Add feature" not "Added feature")
- First line should be ≤50 characters
- Body should wrap at 72 characters
- Reference issues and pull requests
- Explain **why**, not just **what**

## Pull Request Process

### Before Submitting

- [ ] Code follows the project's coding standards
- [ ] All tests pass (`./run-tests.sh all`)
- [ ] New tests added for new features
- [ ] Documentation updated if needed
- [ ] No merge conflicts with main branch
- [ ] Commit messages follow guidelines

### PR Description Template

```markdown
## Description
Brief description of changes

## Related Issue
Fixes #123

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
How to test these changes:
1. Step 1
2. Step 2
3. Expected result

## Screenshots (if applicable)
[Add screenshots here]

## Checklist
- [ ] My code follows the style guidelines
- [ ] I have performed a self-review
- [ ] I have commented my code where needed
- [ ] I have updated the documentation
- [ ] My changes generate no new warnings
- [ ] I have added tests
- [ ] All tests pass locally
```

### Review Process

1. Submit your PR with a clear description
2. Wait for automated checks to complete
3. Address reviewer feedback promptly
4. Make requested changes in new commits
5. Once approved, a maintainer will merge

### Getting Your PR Merged

- Respond to review comments within 1 week
- Keep PRs focused on a single feature/fix
- Rebase on main if conflicts occur
- Be patient and respectful

## Reporting Bugs

### Before Reporting

1. **Search existing issues** - Check if it's already reported
2. **Use latest version** - Ensure you're on the latest release
3. **Reproduce the bug** - Confirm it's reproducible
4. **Gather information** - Collect logs and system details

### Bug Report Template

```markdown
## Bug Description
Clear description of the bug

## Steps to Reproduce
1. Go to '...'
2. Click on '...'
3. See error

## Expected Behavior
What you expected to happen

## Actual Behavior
What actually happened

## Environment
- OS: [e.g., Ubuntu 22.04]
- Go Version: [e.g., 1.21]
- Node.js Version: [e.g., 18.17]
- MySQL Version: [e.g., 8.0]
- Browser (if frontend): [e.g., Chrome 120]

## Logs
```
Paste relevant log output here
```

## Screenshots
If applicable, add screenshots
```

## Suggesting Features

### Feature Request Template

```markdown
## Feature Description
Clear description of the proposed feature

## Problem It Solves
What problem does this solve?

## Proposed Solution
How would you implement this?

## Alternatives Considered
What other solutions did you consider?

## Additional Context
Any other context, screenshots, or examples
```

### Feature Discussion

- Open an issue with the `enhancement` label
- Discuss the feature with maintainers
- Wait for approval before starting work
- Consider backwards compatibility
- Think about performance implications

## Development Tips

### Useful Commands

```bash
# Format Go code
gofmt -w .

# Run linter
golint ./...

# Check for common mistakes
go vet ./...

# Format TypeScript
npm run format

# Run linter
npm run lint

# Type checking
npm run type-check
```

### Debugging

```bash
# Debug Go application
dlv debug ./main.go

# Check mail logs
tail -f /var/log/mail.log

# Test Postfix configuration
postfix check

# Test Dovecot configuration
doveconf -n
```

### Performance Testing

```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Profile CPU usage
go test -cpuprofile cpu.prof -bench .
go tool pprof cpu.prof

# Profile memory
go test -memprofile mem.prof -bench .
go tool pprof mem.prof
```

## Community

### Getting Help

- **GitHub Issues** - For bugs and feature requests
- **Discussions** - For questions and general discussion
- **Documentation** - Check [deployment/README.md](deployment/README.md)

### Recognition

Contributors will be recognized in:
- Release notes
- CONTRIBUTORS.md file
- GitHub contributors page

## License

By contributing to this project, you agree that your contributions will be licensed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for details.

---

Thank you for contributing to Mail System Groupware Suite! 🎉
