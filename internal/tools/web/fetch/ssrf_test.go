package fetch_test

import (
	"net"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/tools/web/fetch"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		// IPv4 Loopback
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		// IPv4 Private (RFC 1918)
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"192.168.255.255", true},
		// IPv4 Link-local & Broadcast & Special
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"100.64.0.1", true},   // CGNAT
		{"192.0.2.1", true},    // TEST-NET-1
		{"198.51.100.1", true}, // TEST-NET-2
		{"203.0.113.1", true},  // TEST-NET-3
		{"224.0.0.1", true},    // Multicast
		// IPv6 Loopback & Link-local & Unique local
		{"::1", true},
		{"::", true},
		{"fc00::1", true},
		{"fd00::1", true},
		{"fe80::1", true},
		{"ff02::1", true},
		// IPv4-mapped IPv6
		{"::ffff:127.0.0.1", true},
		{"::ffff:10.0.0.1", true},
		{"::ffff:192.168.1.1", true},
		// Public IPv4
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
		{"142.250.190.46", false},
		// Public IPv6
		{"2606:4700:4700::1111", false},
		{"2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("invalid test IP: %s", tt.ip)
			}
			got := fetch.IsPrivateIP(ip)
			if got != tt.expected {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.expected)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		allowLocal  bool
		expectError bool
	}{
		{
			name:        "valid public https URL",
			rawURL:      "https://example.com/article",
			allowLocal:  false,
			expectError: false,
		},
		{
			name:        "valid public http URL",
			rawURL:      "http://example.org/page?query=1",
			allowLocal:  false,
			expectError: false,
		},
		{
			name:        "disallowed file scheme",
			rawURL:      "file:///etc/passwd",
			allowLocal:  false,
			expectError: true,
		},
		{
			name:        "disallowed ftp scheme",
			rawURL:      "ftp://example.com/file.txt",
			allowLocal:  false,
			expectError: true,
		},
		{
			name:        "disallowed gopher scheme",
			rawURL:      "gopher://example.com",
			allowLocal:  false,
			expectError: true,
		},
		{
			name:        "empty URL",
			rawURL:      "",
			allowLocal:  false,
			expectError: true,
		},
		{
			name:        "loopback IP disallowed",
			rawURL:      "http://127.0.0.1:8080/secret",
			allowLocal:  false,
			expectError: true,
		},
		{
			name:        "private IP disallowed",
			rawURL:      "http://192.168.1.100/admin",
			allowLocal:  false,
			expectError: true,
		},
		{
			name:        "loopback IP allowed when allowLocal is true",
			rawURL:      "http://127.0.0.1:8080/test",
			allowLocal:  true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fetch.ValidateURL(tt.rawURL, tt.allowLocal)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateURL(%q, %v) error = %v, want error: %v", tt.rawURL, tt.allowLocal, err, tt.expectError)
			}
		})
	}
}
