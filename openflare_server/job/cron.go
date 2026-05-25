package job

import (
	"github.com/robfig/cron/v3"
	"log/slog"
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

	cronRunner.Start()
}

func StopCronJobs() {
	if cronRunner != nil {
		cronRunner.Stop()
	}
}
