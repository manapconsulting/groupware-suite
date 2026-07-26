package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Maildir layout on disk matches Postfix virtual_mailbox_base and the Dovecot
// mail_location: /var/mail/vhosts/<domain>/<localpart>/Maildir
const (
	defaultVhostsDir = "/var/mail/vhosts"
	defaultVmailUID  = 5000
	defaultVmailGID  = 5000
)

func vhostsDir() string {
	if dir := os.Getenv("MAIL_VHOSTS_DIR"); dir != "" {
		return dir
	}
	return defaultVhostsDir
}

func vmailIDs() (int, int) {
	uid, gid := defaultVmailUID, defaultVmailGID
	if v, err := strconv.Atoi(os.Getenv("VMAIL_UID")); err == nil {
		uid = v
	}
	if v, err := strconv.Atoi(os.Getenv("VMAIL_GID")); err == nil {
		gid = v
	}
	return uid, gid
}

// MaildirPath returns the Maildir directory for an email address.
// The caller must have validated the address first (see validateEmail).
func MaildirPath(email string) (string, error) {
	local, domain, err := splitAddress(email)
	if err != nil {
		return "", err
	}
	return filepath.Join(vhostsDir(), domain, local, "Maildir"), nil
}

// CreateMaildir creates the Maildir skeleton (cur/new/tmp) for a new mailbox
// and hands ownership to the vmail user Dovecot runs mailboxes as.
func CreateMaildir(email string) error {
	maildir, err := MaildirPath(email)
	if err != nil {
		return err
	}

	uid, gid := vmailIDs()

	// Every level from the domain dir down must be owned by vmail, so walk the
	// path instead of a single MkdirAll.
	paths := []string{
		filepath.Dir(filepath.Dir(maildir)), // /var/mail/vhosts/<domain>
		filepath.Dir(maildir),               // .../<localpart>
		maildir,
		filepath.Join(maildir, "cur"),
		filepath.Join(maildir, "new"),
		filepath.Join(maildir, "tmp"),
	}

	for _, p := range paths {
		if err := os.MkdirAll(p, 0770); err != nil {
			return fmt.Errorf("create %s: %w", p, err)
		}
		// Best effort: chown fails when the API does not run as root, in which
		// case Dovecot's own auto-create still works from a correct parent.
		if err := os.Chown(p, uid, gid); err != nil && !os.IsPermission(err) {
			return fmt.Errorf("chown %s: %w", p, err)
		}
		if err := os.Chmod(p, 0770); err != nil {
			return fmt.Errorf("chmod %s: %w", p, err)
		}
	}

	return nil
}

// MoveMaildir relocates an existing mailbox after its address changed. A
// missing source is not an error: the account may predate maildir creation, in
// which case the new location is created empty.
func MoveMaildir(oldEmail, newEmail string) error {
	oldMaildir, err := MaildirPath(oldEmail)
	if err != nil {
		return err
	}
	newMaildir, err := MaildirPath(newEmail)
	if err != nil {
		return err
	}

	oldHome, newHome := filepath.Dir(oldMaildir), filepath.Dir(newMaildir)

	if _, err := os.Stat(oldHome); os.IsNotExist(err) {
		return CreateMaildir(newEmail)
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(newHome); err == nil {
		return fmt.Errorf("%s already exists", newHome)
	} else if !os.IsNotExist(err) {
		return err
	}

	uid, gid := vmailIDs()
	newDomainDir := filepath.Dir(newHome)
	if err := os.MkdirAll(newDomainDir, 0770); err != nil {
		return fmt.Errorf("create %s: %w", newDomainDir, err)
	}
	if err := os.Chown(newDomainDir, uid, gid); err != nil && !os.IsPermission(err) {
		return fmt.Errorf("chown %s: %w", newDomainDir, err)
	}

	return os.Rename(oldHome, newHome)
}

// RemoveMaildir deletes a mailbox directory. Only used to unwind a failed
// account creation, never for existing accounts.
func RemoveMaildir(email string) error {
	maildir, err := MaildirPath(email)
	if err != nil {
		return err
	}
	// Remove the user directory (parent of Maildir), not the domain directory.
	return os.RemoveAll(filepath.Dir(maildir))
}
