package job

import (
	"log/slog"
	"openflare/model"
	"openflare/service"
	"time"
)

type SSLRenewJob struct {
}

func (j *SSLRenewJob) Run() {
	slog.Info("The scheduled certificate update task is currently in progress ...")

	certificates, err := model.ListTLSCertificates()
	if err != nil {
		slog.Error("failed to list certificates in SSL renew job", "error", err)
		return
	}

	now := time.Now()
	for _, cert := range certificates {
		if !cert.AutoRenew || cert.Provider != "acme" || cert.ApplyStatus == "applying" {
			continue
		}

		sub := cert.NotAfter.Sub(now)
		// Expiring in less than 7 days (7 * 24 hours)
		if sub.Hours() < 168 {
			slog.Info("Update the SSL certificate for the domain", "domain", cert.PrimaryDomain)

			// Invoke renew process (async go-routine handles Lego inside)
			_, err := service.RenewTLSCertificate(cert.ID)
			if err != nil {
				slog.Error("Failed to update the SSL certificate", "domain", cert.PrimaryDomain, "error", err)
				continue
			}
			slog.Info("Triggered the SSL certificate renew for domain", "domain", cert.PrimaryDomain)
		}
	}
	slog.Info("The scheduled certificate update task has completed")
}
