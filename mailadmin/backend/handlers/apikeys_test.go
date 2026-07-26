package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		key, prefix, err := generateAPIKey()
		if err != nil {
			t.Fatalf("generateAPIKey: %v", err)
		}
		if !strings.HasPrefix(key, apiKeyLabel) {
			t.Errorf("key %q missing label %q", key, apiKeyLabel)
		}
		if !strings.HasPrefix(key, prefix) || len(prefix) > 16 {
			t.Errorf("prefix %q not a valid display prefix of %q", prefix, key)
		}
		if seen[key] {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = true
	}
}

func TestHashAPIKey(t *testing.T) {
	h1 := hashAPIKey("vmk_secret")
	h2 := hashAPIKey("vmk_secret")
	if h1 != h2 {
		t.Errorf("hash not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(h1))
	}
	if hashAPIKey("other") == h1 {
		t.Errorf("different inputs produced same hash")
	}
}

func TestValidateSources(t *testing.T) {
	ok := [][]string{
		{"192.0.2.1"},
		{"10.0.0.0/8"},
		{"mail.example.com"},
		{"192.0.2.1", "10.0.0.0/8", "host.example.org"},
		{"2001:db8::1"},
	}
	for _, in := range ok {
		if _, err := validateSources(in); err != nil {
			t.Errorf("validateSources(%v) unexpected error: %v", in, err)
		}
	}

	bad := [][]string{
		nil,
		{},
		{"   "},
		{"not_a_host!"},
		{"999.999.999.999"},
		{"10.0.0.0/99"},
	}
	for _, in := range bad {
		if _, err := validateSources(in); err == nil {
			t.Errorf("validateSources(%v) expected error, got nil", in)
		}
	}
}

// ProvisionCreateUser must reject an address outside the key's domain before it
// ever touches the database, so this runs without a DB connection.
func TestProvisionCreateUserRejectsForeignDomain(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"email":    "someone@attacker.com",
		"password": "hunter2hunter2",
	})
	r := httptest.NewRequest("POST", "/api/provision/users", strings.NewReader(string(body)))
	ctx := context.WithValue(r.Context(), ctxKeyDomainID, 42)
	ctx = context.WithValue(ctx, ctxKeyDomainName, "vendisens.com")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	ProvisionCreateUser(w, r)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400 for foreign-domain address", w.Code)
	}
	if !strings.Contains(w.Body.String(), "vendisens.com") {
		t.Errorf("expected domain-scope message, got %q", w.Body.String())
	}
}

func TestProvisionCreateUserRequiresKeyContext(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/provision/users", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	ProvisionCreateUser(w, r)

	if w.Code != 500 {
		t.Fatalf("status = %d, want 500 when key context missing", w.Code)
	}
}
