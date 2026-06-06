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
