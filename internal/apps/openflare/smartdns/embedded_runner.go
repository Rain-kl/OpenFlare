// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	defaultDNSListenAddr  = ":53"
	colInstanceStatus     = "status"
	colInstanceLastError  = "last_error"
	instanceTokenBytes    = 24
	instanceIDRandomBytes = 4
	discoveryTokenBytes   = 16
	embeddedGoVersion     = "embedded-go"
	subprocessMaxBackoff  = 30 * time.Second
)

// StartEmbeddedRunner boots the local authoritative SmartDNS worker (Rust binary subprocess when configured,
// falling back seamlessly to the built-in pure-Go miekg/dns server).
func StartEmbeddedRunner(ctx context.Context) {
	if config.Config == nil || !config.Config.SmartDNS.Enabled {
		return
	}
	settings := readSmartDNSSettingsMap(ctx)
	if !settings.Enabled {
		logger.InfoF(ctx, "[SmartDNS] disabled via system setting, embedded runner skipped")
		return
	}

	listenAddr := strings.TrimSpace(config.Config.SmartDNS.ListenAddr)
	if listenAddr == "" {
		listenAddr = defaultDNSListenAddr
	}
	embeddedName := strings.TrimSpace(config.Config.SmartDNS.EmbeddedName)
	if embeddedName == "" {
		embeddedName = "Embedded SmartDNS"
	}

	token, err := generateHexToken(instanceTokenBytes)
	if err != nil {
		logger.ErrorF(ctx, "[SmartDNS] generate embedded instance token failed: %v", err)
		return
	}

	inst, err := repository.EnsureEmbeddedDNSInstance(ctx, &model.OpenFlareDNSInstance{
		InstanceID:  embeddedInstanceID,
		Name:        embeddedName,
		Mode:        model.DNSInstanceModeEmbedded,
		ListenAddr:  listenAddr,
		AccessToken: token,
		Version:     "embedded",
		Status:      model.DNSInstanceStatusPending,
	})
	if err != nil {
		logger.ErrorF(ctx, "[SmartDNS] register embedded instance failed: %v", err)
		return
	}

	binaryPath := strings.TrimSpace(config.Config.SmartDNS.BinaryPath)
	if binaryPath != "" {
		if info, statErr := os.Stat(binaryPath); statErr == nil && !info.IsDir() {
			startRustSubprocessSupervisor(ctx, binaryPath, inst, listenAddr)
			return
		}
		logger.WarnF(ctx, "[SmartDNS] configured binary %q not found, falling back to embedded Go DNS", binaryPath)
	}

	srv := NewEmbeddedGoDNSServer(listenAddr, defaultSnapshotManager)
	if err := srv.Start(ctx); err != nil {
		logger.ErrorF(ctx, "[SmartDNS] failed to start embedded Go DNS on %s: %v", listenAddr, err)
		_ = repository.UpdateDNSInstanceRuntime(ctx, embeddedInstanceID, map[string]any{
			colInstanceStatus:    model.DNSInstanceStatusOffline,
			colInstanceLastError: err.Error(),
		})
		return
	}

	now := time.Now().UTC()
	_ = repository.UpdateDNSInstanceRuntime(ctx, embeddedInstanceID, map[string]any{
		colInstanceStatus:    model.DNSInstanceStatusOnline,
		"version":            embeddedGoVersion,
		"listen_addr":        srv.Addr(),
		"last_seen_at":       &now,
		colInstanceLastError: "",
	})
	lifecycle.OnShutdown("smartdns embedded go server", srv.Stop)
	logger.InfoF(ctx, "[SmartDNS] embedded Go DNS listening on %s", srv.Addr())
}

func startRustSubprocessSupervisor(ctx context.Context, binaryPath string, inst *model.OpenFlareDNSInstance, listenAddr string) {
	subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	lifecycle.OnShutdown("smartdns rust subprocess", func(context.Context) error {
		cancel()
		return nil
	})

	controllerAddr := strings.TrimSpace(config.Config.App.Addr)
	if strings.HasPrefix(controllerAddr, ":") {
		controllerAddr = "127.0.0.1" + controllerAddr
	}
	controllerURL := "http://" + controllerAddr

	go func() {
		backoff := time.Second
		for {
			if subCtx.Err() != nil {
				return
			}
			cmd := exec.CommandContext( //nolint:gosec // operator-configured local binary path
				subCtx,
				binaryPath,
				"serve",
				"--controller-url", controllerURL,
				"--instance-id", inst.InstanceID,
				"--token", inst.AccessToken,
				"--listen", listenAddr,
			)
			logger.InfoF(subCtx, "[SmartDNS] starting openflare-dns subprocess %q on %s", binaryPath, listenAddr)
			err := cmd.Run()
			if subCtx.Err() != nil {
				return
			}
			if err != nil {
				logger.WarnF(subCtx, "[SmartDNS] openflare-dns exited unexpectedly (%v); restarting in %s", err, backoff)
				_ = repository.UpdateDNSInstanceRuntime(subCtx, inst.InstanceID, map[string]any{
					colInstanceStatus:    model.DNSInstanceStatusOffline,
					colInstanceLastError: err.Error(),
				})
			}
			select {
			case <-subCtx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < subprocessMaxBackoff {
				backoff *= 2
			}
		}
	}()
}
