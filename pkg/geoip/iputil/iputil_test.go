// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package iputil

import (
	"net"
	"testing"
)

func TestNormalizeIP(t *testing.T) {
	if got := NormalizeIP(" 8.8.8.8 "); got != "8.8.8.8" {
		t.Fatalf("unexpected normalized ipv4: %q", got)
	}
	if got := NormalizeIP("[::1]"); got != "" {
		t.Fatalf("expected invalid bracketed host to be rejected, got %q", got)
	}
}

func TestNormalizeRemoteAddr(t *testing.T) {
	if got := NormalizeRemoteAddr("203.0.113.10:8443"); got != "203.0.113.10" {
		t.Fatalf("unexpected remote addr normalization: %q", got)
	}
}

func TestIsPublic(t *testing.T) {
	if !IsPublic(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected public ip to be detected")
	}
	if IsPublic(net.ParseIP("10.0.0.8")) {
		t.Fatal("expected private ip to be rejected")
	}
	if IsPublic(net.ParseIP("127.0.0.1")) {
		t.Fatal("expected loopback ip to be rejected")
	}
}

func TestScore(t *testing.T) {
	if got := Score(net.ParseIP("8.8.8.8")); got != 2 {
		t.Fatalf("unexpected score for public ip: %d", got)
	}
	if got := Score(net.ParseIP("10.0.0.8")); got != 1 {
		t.Fatalf("unexpected score for private ip: %d", got)
	}
	if got := Score(net.ParseIP("127.0.0.1")); got != -1 {
		t.Fatalf("unexpected score for loopback ip: %d", got)
	}
}

func TestIsPublicRoutable(t *testing.T) {
	publicCases := []string{
		"1.1.1.1",
		"8.8.8.8",
		"82.22.26.203",
		"172.32.0.1",
		"100.128.0.1",
		"::ffff:8.8.8.8",
		"2606:4700:4700::1111",
		"2001:4860:4860::8888",
	}
	for _, raw := range publicCases {
		if !IsPublicRoutableString(raw) {
			t.Errorf("expected %s to be public routable", raw)
		}
	}

	excludedCases := []string{
		"",
		"not-an-ip",
		"0.0.0.0",
		"0.1.2.3",
		"10.0.0.1",
		"100.64.0.1",
		"100.127.255.254",
		"127.0.0.1",
		"169.254.169.254",
		"172.16.0.1",
		"172.27.0.1",
		"172.31.255.254",
		"192.0.0.1",
		"192.0.2.1",
		"192.88.99.1",
		"192.168.1.1",
		"198.18.0.1",
		"198.19.255.254",
		"198.51.100.1",
		"203.0.113.10",
		"224.0.0.1",
		"240.0.0.1",
		"255.255.255.255",
		"::",
		"::1",
		"::ffff:172.27.0.1",
		"64:ff9b::1.2.3.4",
		"100::1",
		"2001::1",
		"2001:2::1",
		"2001:db8::1",
		"2002::1",
		"3fff::1",
		"fc00::1",
		"fd12:3456::1",
		"fe80::1",
		"ff02::1",
	}
	for _, raw := range excludedCases {
		if IsPublicRoutableString(raw) {
			t.Errorf("expected %q to be excluded from public routable IPs", raw)
		}
	}
}
