package heartbeat

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/Rain-kl/Wavelet/internal/flared/config"
	"github.com/Rain-kl/Wavelet/internal/flared/frpc"
	"github.com/Rain-kl/Wavelet/internal/flared/httpclient"
	"github.com/Rain-kl/Wavelet/internal/flared/updater"
	"github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

var (
	lookupOutboundIP = geoip.GetOutboundIP
	lookupLocalIP    = detectLocalNodeIP
)

type Service struct {
	client      *httpclient.Client
	frpcManager *frpc.Manager
	config      *config.Config
	updater     *updater.Service
}

func New(client *httpclient.Client, manager *frpc.Manager, cfg *config.Config) *Service {
	return &Service{
		client:      client,
		frpcManager: manager,
		config:      cfg,
		updater:     updater.New(),
	}
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.config.HeartbeatInterval.Duration())
	defer ticker.Stop()

	// initial heartbeat
	s.doHeartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.doHeartbeat(ctx)
		}
	}
}

func (s *Service) doHeartbeat(ctx context.Context) {
	slog.Debug("sending flared heartbeat")

	ip := detectNodeIP()

	payload := service.FlaredHeartbeatPayload{
		ClientVersion:   config.Version,
		FrpVersion:      s.frpcManager.GetVersion(),
		IP:              ip,
		TunnelStatus:    "running", // TODO implement proper status tracking
		ConnectedRelays: s.frpcManager.GetConnectedRelays(),
		CurrentVersion:  s.frpcManager.GetCurrentConfigVersion(),
		CurrentChecksum: s.frpcManager.GetCurrentConfigChecksum(),
	}

	resp, err := s.client.Heartbeat(ctx, payload)
	if err != nil {
		slog.Error("flared heartbeat failed", "error", err)
		return
	}
	slog.Debug("flared heartbeat succeeded")

	if resp != nil && resp.TunnelSettings != nil {
		s.tryAutoUpdate(ctx, resp.TunnelSettings)
	}
}

func (s *Service) tryAutoUpdate(ctx context.Context, settings *service.RelaySettings) {
	if settings == nil || s.updater == nil {
		return
	}
	force := settings.UpdateNow
	shouldCheck := settings.AutoUpdate || force
	if !shouldCheck || settings.UpdateRepo == "" {
		return
	}
	channel := "stable"
	if force && settings.UpdateChannel != "" {
		channel = settings.UpdateChannel
	}
	slog.Info("checking for client updates", "repo", settings.UpdateRepo, "channel", channel, "force", force)
	err := s.updater.CheckAndUpdate(ctx, settings.UpdateRepo, updater.UpdateOptions{
		Channel: channel,
		TagName: settings.UpdateTag,
		Force:   force,
	})
	if err != nil {
		slog.Error("client update check failed", "error", err)
	}
}

func detectNodeIP() string {
	if ip := detectOutboundNodeIP(); ip != "" {
		return ip
	}
	return lookupLocalIP()
}

func detectOutboundNodeIP() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ip, err := lookupOutboundIP(ctx)
	if err != nil || ip == nil {
		return ""
	}
	return ip.String()
}

func detectLocalNodeIP() string {
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
