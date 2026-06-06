package job

import (
	"log/slog"
	"sync"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/service"
)

var lastUptimeKumaSyncTime time.Time
var uptimeKumaSyncMutex sync.Mutex

type UptimeKumaSyncJob struct{}

func (j *UptimeKumaSyncJob) Run() {
	if !common.UptimeKumaEnabled {
		return
	}

	interval := common.UptimeKumaSyncInterval
	if interval <= 0 {
		interval = 5
	}

	if time.Since(lastUptimeKumaSyncTime) < time.Duration(interval)*time.Minute {
		return
	}

	if !uptimeKumaSyncMutex.TryLock() {
		slog.Warn("Uptime Kuma sync job is already running, skipping this scheduled run")
		return
	}
	defer uptimeKumaSyncMutex.Unlock()

	slog.Info("Starting scheduled Uptime Kuma sync")
	if err := service.SyncToUptimeKuma(); err != nil {
		slog.Error("Uptime Kuma sync failed", "error", err)
	} else {
		lastUptimeKumaSyncTime = time.Now()
		slog.Info("Uptime Kuma sync completed successfully")
	}
}
