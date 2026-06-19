package nodeip

import (
	"context"
	"net"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
)

var (
	LookupOutboundIP = geoip.GetOutboundIP
	LookupLocalIP    = DetectLocal
)

func Detect() string {
	if ip := detectOutbound(); ip != "" {
		return ip
	}
	return LookupLocalIP()
}

func detectOutbound() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ip, err := LookupOutboundIP(ctx)
	if err != nil || ip == nil {
		return ""
	}
	return ip.String()
}

func DetectLocal() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	bestIP := ""
	bestPriority := -1
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
				continue
			}
			ipv4 := ipNet.IP.To4()
			if ipv4 == nil {
				continue
			}
			priority := iputil.Score(ipv4)
			if priority > bestPriority {
				bestIP = ipv4.String()
				bestPriority = priority
			}
			if bestPriority == 2 {
				return bestIP
			}
		}
	}
	return bestIP
}