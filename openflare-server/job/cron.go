package job

import (
	"log/slog"

	"github.com/robfig/cron/v3"
)

var cronRunner *cron.Cron

func InitCronJobs() {
	cronRunner = cron.New()

	// Register SSL renew job
	_, err := cronRunner.AddJob("0 0 * * *", &SSLRenewJob{})
	if err != nil {
		slog.Error("failed to register SSL renew cron job", "error", err)
	} else {
		slog.Info("registered SSL renew cron job")
	}

	_, err = cronRunner.AddJob("@every 5m", &WAFIPGroupSyncJob{})
	if err != nil {
		slog.Error("failed to register WAF IP group sync cron job", "error", err)
	} else {
		slog.Info("registered WAF IP group sync cron job")
	}

	// Register Uptime Kuma sync job (check every minute)
	_, err = cronRunner.AddJob("* * * * *", &UptimeKumaSyncJob{})
	if err != nil {
		slog.Error("failed to register Uptime Kuma sync cron job", "error", err)
	} else {
		slog.Info("registered Uptime Kuma sync cron job")
	}

	cronRunner.Start()
}

func StopCronJobs() {
	if cronRunner != nil {
		cronRunner.Stop()
	}
}
