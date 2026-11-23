// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls/acme"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	// SSLSingleRenewTask renews a single ACME TLS certificate.
	SSLSingleRenewTask = "openflare:ssl_single_renew"
	// TaskTypeSSLSingleRenew is the admin task type for single SSL renewal.
	TaskTypeSSLSingleRenew = "of_ssl_single_renew"
)

// SSLSingleRenewMeta describes the single SSL renewal task.
var SSLSingleRenewMeta = task.TaskMeta{
	Type:         TaskTypeSSLSingleRenew,
	AsynqTask:    SSLSingleRenewTask,
	Name:         "OpenFlare single-certificate SSL renewal",
	Description:  "Renew a single specified ACME certificate",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "id",
			Label:       "Certificate ID",
			Type:        "number",
			Required:    true,
			Placeholder: "Enter the certificate ID",
			Description: "ID of the TLS certificate to renew",
		},
		{
			Name:        "revoke_previous",
			Label:       "Revoke Previous Certificate",
			Type:        "boolean",
			Required:    false,
			Description: "Revoke the previously issued ACME certificate for key compromise after issuing a replacement",
		},
	},
}

// SSLSingleRenewPayload is the payload structure for SSLSingleRenewTask.
type SSLSingleRenewPayload struct {
	ID              uint   `json:"id"`
	RevokePrevious  bool   `json:"revoke_previous,omitempty"`
	PreviousCertPEM string `json:"previous_cert_pem,omitempty"`
}

// SSLSingleRenewHandler renews a specific TLS certificate.
type SSLSingleRenewHandler struct{}

// ValidatePayload validates and normalizes the task payload.
func (h *SSLSingleRenewHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("task parameters must not be empty")
	}

	var req SSLSingleRenewPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	if req.ID == 0 {
		return nil, errors.New("certificate ID must be nonzero")
	}

	return json.Marshal(req)
}

// Execute runs the certificate renewal for the specified ID.
func (h *SSLSingleRenewHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	var req SSLSingleRenewPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse task parameters: %w", err)
	}

	task.AppendLog(ctx, "Starting certificate renewal, ID: %d", req.ID)

	cert, err := repository.GetTLSCertificateByID(ctx, req.ID)
	if err != nil {
		task.AppendLog(ctx, "Failed to get certificate record ID=%d: %v", req.ID, err)
		return nil, fmt.Errorf("failed to get certificate record: %w", err)
	}

	if cert.Provider != tlsProviderACME {
		task.AppendLog(ctx, "Certificate %s (ID=%d) is not ACME-managed and cannot be renewed automatically (provider: %s)", cert.PrimaryDomain, req.ID, cert.Provider)
		return nil, fmt.Errorf("certificate %s is not ACME-managed and cannot be renewed automatically", cert.PrimaryDomain)
	}

	oldCertPEM := strings.TrimSpace(req.PreviousCertPEM)
	if oldCertPEM == "" {
		oldCertPEM = strings.TrimSpace(cert.CertPEM)
	}
	alreadyReplaced := req.RevokePrevious &&
		strings.TrimSpace(req.PreviousCertPEM) != "" &&
		strings.TrimSpace(cert.CertPEM) != "" &&
		edgeconfig.CertificatePEMDigest(cert.CertPEM) != edgeconfig.CertificatePEMDigest(oldCertPEM)
	if !alreadyReplaced {
		task.AppendLog(ctx, "Preparing to issue or renew a certificate for domains [%s]", cert.PrimaryDomain)
		if err := obtainTLSCertificate(ctx, cert); err != nil {
			task.AppendLog(ctx, "Certificate issuance failed: %v", err)
			return nil, err
		}
	} else {
		task.AppendLog(ctx, "Replacement certificate already issued; retrying previous certificate revocation")
	}

	if req.RevokePrevious && oldCertPEM != "" {
		task.AppendLog(ctx, "Revoking previously issued ACME certificate (reason: keyCompromise)...")
		if revokeErr := revokeTLSCertificate(ctx, cert, oldCertPEM, acme.CRLReasonKeyCompromise); revokeErr != nil {
			task.AppendLog(ctx, "Failed to revoke previous certificate: %v", revokeErr)
			return nil, fmt.Errorf("replacement certificate issued, but failed to revoke previous certificate: %w", revokeErr)
		}
		task.AppendLog(ctx, "Previous ACME certificate revoked successfully")
	}

	msg := fmt.Sprintf("Certificate %s renewed successfully", cert.PrimaryDomain)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
