package iputil

import (
	"net"
	"strings"
)

func NormalizeIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return ip.String()
}

func NormalizeRemoteAddr(remoteAddr string) string {
	trimmed := strings.TrimSpace(remoteAddr)
	if trimmed == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return NormalizeIP(host)
	}
	return NormalizeIP(trimmed)
}

func IsPublic(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func IsPublicString(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return IsPublic(ip)
}

func Score(ip net.IP) int {
	if ip == nil {
		return -1
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
		return -1
	}
	if IsPublic(ip) {
		return 2
	}
	return 1
}
