package handlers

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestSourceAllowed(t *testing.T) {
	// Stub DNS so FQDN entries resolve deterministically.
	orig := hostResolver
	hostResolver = func(host string) ([]net.IP, error) {
		switch host {
		case "allowed.example.com":
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		case "nomatch.example.com":
			return []net.IP{net.ParseIP("198.51.100.1")}, nil
		default:
			return nil, &net.DNSError{Err: "not found", Name: host}
		}
	}
	defer func() {
		hostResolver = orig
		dnsCache = map[string]dnsCacheEntry{}
	}()

	cases := []struct {
		name    string
		ip      string
		entries []string
		want    bool
	}{
		{"exact ip match", "192.0.2.5", []string{"192.0.2.5"}, true},
		{"exact ip mismatch", "192.0.2.6", []string{"192.0.2.5"}, false},
		{"cidr contains", "10.1.2.3", []string{"10.1.0.0/16"}, true},
		{"cidr excludes", "10.2.2.3", []string{"10.1.0.0/16"}, false},
		{"fqdn resolves and matches", "203.0.113.10", []string{"allowed.example.com"}, true},
		{"fqdn resolves no match", "203.0.113.11", []string{"nomatch.example.com"}, false},
		{"fqdn unresolvable", "203.0.113.10", []string{"broken.example.com"}, false},
		{"multiple entries one matches", "192.0.2.5", []string{"10.0.0.0/8", "192.0.2.5"}, true},
		{"empty allowlist denies", "192.0.2.5", nil, false},
		{"malformed entry ignored", "192.0.2.5", []string{"not an ip", "192.0.2.5"}, true},
		{"ipv6 match", "2001:db8::1", []string{"2001:db8::1"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("bad test IP %q", tc.ip)
			}
			if got := sourceAllowed(ip, tc.entries); got != tc.want {
				t.Errorf("sourceAllowed(%s, %v) = %v, want %v", tc.ip, tc.entries, got, tc.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	// Non-loopback peer: forwarded headers are forgeable and must be ignored.
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("X-Real-IP", "5.6.7.8")
	if ip := clientIP(r); ip == nil || ip.String() != "203.0.113.7" {
		t.Fatalf("non-loopback clientIP = %v, want 203.0.113.7 (headers ignored)", ip)
	}

	// Loopback peer (local nginx): trust X-Real-IP it set.
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "127.0.0.1:40000"
	r2.Header.Set("X-Real-IP", "198.51.100.9")
	r2.Header.Set("X-Forwarded-For", "198.51.100.9, 127.0.0.1")
	if ip := clientIP(r2); ip == nil || ip.String() != "198.51.100.9" {
		t.Fatalf("loopback clientIP = %v, want 198.51.100.9 (from X-Real-IP)", ip)
	}

	// Loopback peer with only X-Forwarded-For: use the first hop.
	r3 := httptest.NewRequest("GET", "/", nil)
	r3.RemoteAddr = "127.0.0.1:40001"
	r3.Header.Set("X-Forwarded-For", "203.0.113.44, 127.0.0.1")
	if ip := clientIP(r3); ip == nil || ip.String() != "203.0.113.44" {
		t.Fatalf("loopback XFF clientIP = %v, want 203.0.113.44", ip)
	}

	// Loopback peer with no forwarded headers: a genuinely local caller.
	r4 := httptest.NewRequest("GET", "/", nil)
	r4.RemoteAddr = "127.0.0.1:40002"
	if ip := clientIP(r4); ip == nil || ip.String() != "127.0.0.1" {
		t.Fatalf("bare loopback clientIP = %v, want 127.0.0.1", ip)
	}
}

func TestSplitSources(t *testing.T) {
	got := splitSources(" 192.0.2.1, 10.0.0.0/8 ,\nhost.example.com\r\n")
	want := []string{"192.0.2.1", "10.0.0.0/8", "host.example.com"}
	if len(got) != len(want) {
		t.Fatalf("splitSources len = %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}
