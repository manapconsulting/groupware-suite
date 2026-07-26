package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitAddress(t *testing.T) {
	valid := []string{"user@example.com", "first.last@mail.example.com", "a+b_c%d-e@example.co.uk"}
	for _, email := range valid {
		if _, _, err := splitAddress(email); err != nil {
			t.Errorf("splitAddress(%q) rejected a valid address: %v", email, err)
		}
	}

	invalid := []string{
		"",
		"nodomain",
		"two@at@example.com",
		"../escape@example.com",
		"path/traversal@example.com",
		".leading@example.com",
		"trailing.@example.com",
		"double..dot@example.com",
		"user@nodot",
		"user@-example.com",
	}
	for _, email := range invalid {
		if _, _, err := splitAddress(email); err == nil {
			t.Errorf("splitAddress(%q) accepted an invalid address", email)
		}
	}
}

func TestCreateAndMoveMaildir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("MAIL_VHOSTS_DIR", base)

	if err := CreateMaildir("test.user@example.com"); err != nil {
		t.Fatalf("CreateMaildir: %v", err)
	}

	maildir := filepath.Join(base, "example.com", "test.user", "Maildir")
	for _, sub := range []string{"cur", "new", "tmp"} {
		info, err := os.Stat(filepath.Join(maildir, sub))
		if err != nil {
			t.Fatalf("expected %s/%s: %v", maildir, sub, err)
		}
		if perm := info.Mode().Perm(); perm != 0770 {
			t.Errorf("%s/%s has mode %o, want 770", maildir, sub, perm)
		}
	}

	// Rename within the same domain
	if err := MoveMaildir("test.user@example.com", "renamed@example.com"); err != nil {
		t.Fatalf("MoveMaildir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "renamed", "Maildir")); err == nil {
		t.Error("MoveMaildir wrote outside the domain directory")
	}
	if _, err := os.Stat(filepath.Join(base, "example.com", "renamed", "Maildir", "new")); err != nil {
		t.Fatalf("expected moved maildir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "example.com", "test.user")); !os.IsNotExist(err) {
		t.Error("old mailbox directory still exists after move")
	}

	// Move across domains creates the target domain directory
	if err := MoveMaildir("renamed@example.com", "renamed@other.example"); err != nil {
		t.Fatalf("MoveMaildir across domains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "other.example", "renamed", "Maildir", "cur")); err != nil {
		t.Fatalf("expected maildir under new domain: %v", err)
	}

	// An occupied target must not be clobbered
	if err := CreateMaildir("occupied@example.com"); err != nil {
		t.Fatalf("CreateMaildir: %v", err)
	}
	if err := MoveMaildir("renamed@other.example", "occupied@example.com"); err == nil {
		t.Error("MoveMaildir overwrote an existing mailbox")
	}

	// A source that was never created on disk yields an empty target
	if err := MoveMaildir("ghost@example.com", "revived@example.com"); err != nil {
		t.Fatalf("MoveMaildir with missing source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "example.com", "revived", "Maildir", "new")); err != nil {
		t.Fatalf("expected new empty maildir: %v", err)
	}
}

func TestRemoveMaildir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("MAIL_VHOSTS_DIR", base)

	if err := CreateMaildir("gone@example.com"); err != nil {
		t.Fatalf("CreateMaildir: %v", err)
	}
	if err := RemoveMaildir("gone@example.com"); err != nil {
		t.Fatalf("RemoveMaildir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "example.com", "gone")); !os.IsNotExist(err) {
		t.Error("mailbox directory still exists after removal")
	}
	// The domain directory is shared and must survive
	if _, err := os.Stat(filepath.Join(base, "example.com")); err != nil {
		t.Errorf("domain directory was removed: %v", err)
	}
}
