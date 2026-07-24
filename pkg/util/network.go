package util

import (
	"log/slog"
	"net"
)

// GetIP returns the first private IPv4 address found on the local network interfaces.
func GetIP() (ip string) {
	ips, err := net.InterfaceAddrs()
	if err != nil {
		slog.Error("get interface addresses failed", "error", err)
		return ip
	}

	for _, a := range ips {
		if candidate, ok := privateIPv4FromAddr(a); ok {
			return candidate
		}
	}
	return
}

func privateIPv4FromAddr(addr net.Addr) (string, bool) {
	ipNet, ok := addr.(*net.IPNet)
	if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
		return "", false
	}
	ip := ipNet.IP.String()
	if isPrivateIPv4(ip) {
		return ip, true
	}
	return "", false
}

func isPrivateIPv4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return parsedIP.IsPrivate()
}
