package heartbeat

import (
	"context"
	"log/slog"
	"strings"

	edgeupdater "github.com/Rain-kl/Wavelet/internal/apps/edge/updater"
)

type AutoUpdateSettings struct {
	AutoUpdate    bool
	UpdateNow     bool
	UpdateRepo    string
	UpdateChannel string
	UpdateTag     string
}

func TryAutoUpdate(ctx context.Context, updater *edgeupdater.Service, settings *AutoUpdateSettings, logLabel string) {
	if settings == nil || updater == nil {
		return
	}
	force := settings.UpdateNow
	shouldCheck := settings.AutoUpdate || force
	if !shouldCheck || strings.TrimSpace(settings.UpdateRepo) == "" {
		return
	}
	channel := "stable"
	if force && strings.TrimSpace(settings.UpdateChannel) != "" {
		channel = settings.UpdateChannel
	}
	slog.Info("checking for "+logLabel+" updates", "repo", settings.UpdateRepo, "channel", channel, "force", force)
	err := updater.CheckAndUpdate(ctx, settings.UpdateRepo, edgeupdater.UpdateOptions{
		Channel: channel,
		TagName: settings.UpdateTag,
		Force:   force,
	})
	if err != nil {
		slog.Error(logLabel+" update check failed", "error", err)
	}
}