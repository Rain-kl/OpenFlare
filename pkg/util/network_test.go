package util

import (
	"testing"
)

func TestIsPrivateIPv4(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", false}, // Loopback is not in RFC 1918 private range
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"8.8.8.8", false},
		{"invalid-ip", false},
	}

	for _, tt := range tests {
		result := isPrivateIPv4(tt.ip)
		if result != tt.expected {
			t.Errorf("isPrivateIPv4(%q) = %v, expected %v", tt.ip, result, tt.expected)
		}
	}
}

func TestGetIP(t *testing.T) {
	ip := GetIP()
	// GetIP should return empty if no private IPv4 address is configured, or a valid IP.
	// We just ensure it doesn't panic.
	t.Logf("GetIP returned: %q", ip)
}
