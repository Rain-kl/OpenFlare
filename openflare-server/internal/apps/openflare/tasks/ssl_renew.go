// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

// RunSSLRenewJob renews all TLS certificates that are due for renewal.
func RunSSLRenewJob(ctx context.Context) error {
	logger.InfoF(ctx, "[OpenFlareTasks] SSL renew job started")

	certificates, err := model.ListTLSCertificates(ctx)
	if err != nil {
		logger.ErrorF(ctx, "[OpenFlareTasks] list certificates failed: %v", err)
		return err
	}

	now := time.Now()
	due := tls.CertificatesDueForRenewal(certificates, now)
	if len(due) == 0 {
		logger.InfoF(ctx, "[OpenFlareTasks] SSL renew job completed: no certificates due")
		return nil
	}

	var triggered int
	for _, cert := range due {
		logger.InfoF(ctx, "[OpenFlareTasks] renewing certificate id=%d domain=%s", cert.ID, cert.PrimaryDomain)
		if _, err := tls.RenewCertificate(ctx, cert.ID); err != nil {
			logger.ErrorF(ctx, "[OpenFlareTasks] renew certificate id=%d domain=%s failed: %v", cert.ID, cert.PrimaryDomain, err)
			continue
		}
		triggered++
	}

	logger.InfoF(ctx, "[OpenFlareTasks] SSL renew job completed: triggered=%d eligible=%d", triggered, len(due))
	return nil
}