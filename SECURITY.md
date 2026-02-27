# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of our mail server software seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please DO NOT

- Open a public GitHub issue for security vulnerabilities
- Disclose the vulnerability publicly before it has been addressed

### Please DO

1. **Email us privately** at: security@manapconsulting.com
2. Include the following information:
   - Type of issue (e.g., SQL injection, XSS, authentication bypass)
   - Full paths of source file(s) related to the issue
   - Location of the affected source code (tag/branch/commit or direct URL)
   - Step-by-step instructions to reproduce the issue
   - Proof-of-concept or exploit code (if possible)
   - Impact of the issue, including how an attacker might exploit it

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours
- **Communication**: We will send you regular updates about our progress
- **Timeline**: We aim to release a fix within 30 days for critical vulnerabilities
- **Credit**: We will credit you in the security advisory (unless you prefer to remain anonymous)

## Security Update Process

1. Security issues are reviewed by the maintainers
2. A fix is developed in a private repository
3. A security advisory is prepared
4. The fix is released and the advisory is published
5. Users are notified through:
   - GitHub Security Advisories
   - Repository README
   - Release notes

## Security Best Practices

When deploying this mail server, we recommend:

### Server Security

- Keep all software up to date (Postfix, Dovecot, MySQL, OS)
- Use strong, unique passwords for all accounts
- Enable fail2ban for brute-force protection
- Configure firewall (ufw/iptables) properly
- Disable root SSH login
- Use SSH keys instead of passwords
- Keep port 3306 (MySQL) closed to external access

### Mail Server Security

- Always use SSL/TLS (port 993 for IMAPS, 587 for submission)
- Configure SPF, DKIM, and DMARC records
- Enable sender permission controls
- Regularly review mail logs for suspicious activity
- Limit mail size and connection rates
- Use strong password hashing (SHA512-CRYPT)

### Application Security

- Change default database passwords
- Use environment variables for sensitive configuration
- Enable HTTPS for all web interfaces
- Set proper file permissions (vmail:vmail for mailboxes)
- Regular security audits of custom code
- Keep dependencies updated

### Monitoring

- Monitor authentication failures
- Check for unusual mail queue activity
- Review fail2ban logs regularly
- Set up alerts for disk space and service failures
- Use tools like:
  - MXToolbox for DNS/mail testing
  - SSL Labs for SSL configuration
  - Security scanners for vulnerability assessment

## Known Security Considerations

### Default Configuration

The default configuration files contain placeholder values (CHANGE_ME_*) that **must** be changed before deployment:

- Database passwords
- Hostnames
- SSL certificate paths

**Never deploy with default values!**

### Database Access

- The mailuser database account has full privileges on the mailserver database
- This is required for the application to function
- Ensure MySQL is not accessible from external networks
- Use strong passwords for all database accounts

### File Permissions

Mail files are owned by vmail:vmail (UID/GID 5000). Ensure:
- `/var/mail/vhosts` has proper permissions (770)
- Only vmail user can access mailboxes
- Dovecot configuration files are owned by root
- MySQL configuration files have restricted permissions (600/640)

## Security Checklist

Before going to production:

- [ ] All CHANGE_ME_* placeholders replaced
- [ ] Strong passwords set for all accounts
- [ ] SSL certificates obtained and configured
- [ ] Firewall configured and enabled
- [ ] Fail2ban installed and configured
- [ ] SPF/DKIM/DMARC DNS records added
- [ ] Regular backups configured
- [ ] System updates scheduled
- [ ] Monitoring and alerting set up
- [ ] Security audit completed

## Third-Party Dependencies

This project uses several third-party components:

- **Postfix** - Mail Transfer Agent
- **Dovecot** - IMAP/LMTP server
- **MySQL/MariaDB** - Database
- **Let's Encrypt/Certbot** - SSL certificates
- **Go modules** - See go.mod files
- **npm packages** - See package.json files

We recommend:
- Subscribing to security advisories for these components
- Keeping all dependencies up to date
- Regularly scanning for known vulnerabilities

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE/SANS Top 25](https://cwe.mitre.org/top25/)
- [Postfix Security](http://www.postfix.org/SECURITY_README.html)
- [Dovecot Security](https://doc.dovecot.org/admin_manual/security/)

## Questions?

If you have questions about security that are not sensitive in nature, please open a GitHub issue with the "security" label.

For sensitive security concerns, always email security@manapconsulting.com.

---

**Last Updated**: 2026-02-27
