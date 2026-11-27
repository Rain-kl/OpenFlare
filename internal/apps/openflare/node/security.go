// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"strings"
	"time"

	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const maxQuarantineReasonRunes = 255

// CertificateDeliveryView describes one certificate private-key delivery record for a node.
type CertificateDeliveryView struct {
	CertificateID     uint       `json:"certificate_id"`
	CertificateName   string     `json:"certificate_name"`
	PrimaryDomain     string     `json:"primary_domain"`
	Provider          string     `json:"provider"`
	ApplyStatus       string     `json:"apply_status"`
	NotAfter          *time.Time `json:"not_after,omitempty"`
	CertSHA256        string     `json:"cert_sha256"`
	CurrentCertSHA256 string     `json:"current_cert_sha256"`
	MatchesCurrent    bool       `json:"matches_current"`
	ReleaseSequence   int64      `json:"release_sequence"`
	DeliveryCount     int64      `json:"delivery_count"`
	FirstDeliveredAt  time.Time  `json:"first_delivered_at"`
	LastDeliveredAt   time.Time  `json:"last_delivered_at"`
}

// QuarantineInput configures emergency quarantine actions for a node.
type QuarantineInput struct {
	Reason          string `json:"reason"`
	RotateACMECerts bool   `json:"rotate_acme_certs"`
}

// QuarantineResult summarizes the node quarantine and certificate exposure response.
type QuarantineResult struct {
	Node                       *View                     `json:"node"`
	ExposedCertificates        []CertificateDeliveryView `json:"exposed_certificates"`
	RotatedCertificateIDs      []uint                    `json:"rotated_certificate_ids"`
	RotationErrors             map[uint]string           `json:"rotation_errors"`
	ManualActionCertificateIDs []uint                    `json:"manual_action_certificate_ids"`
	DNSSwitchedGroups          []string                  `json:"dns_switched_groups"`
	DNSUnbackedGroups          []string                  `json:"dns_unbacked_groups"`
}

// ListNodeCertificateDeliveries returns audited memory-key deliveries recorded for a node.
func ListNodeCertificateDeliveries(ctx context.Context, id uint) ([]CertificateDeliveryView, error) {
	ctx = db.WithPrimaryReads(ctx)
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	deliveries, err := repository.ListNodeCertDeliveriesByNodeID(ctx, node.NodeID)
	if err != nil {
		return nil, err
	}
	if len(deliveries) == 0 {
		return []CertificateDeliveryView{}, nil
	}

	certIDSet := make(map[uint]struct{}, len(deliveries))
	certIDs := make([]uint, 0, len(deliveries))
	for _, item := range deliveries {
		if _, ok := certIDSet[item.CertificateID]; ok {
			continue
		}
		certIDSet[item.CertificateID] = struct{}{}
		certIDs = append(certIDs, item.CertificateID)
	}
	certs, err := repository.ListTLSCertificatesByIDs(ctx, certIDs)
	if err != nil {
		return nil, err
	}
	certByID := make(map[uint]model.TLSCertificate, len(certs))
	for _, cert := range certs {
		certByID[cert.ID] = cert
	}

	views := make([]CertificateDeliveryView, 0, len(deliveries))
	for _, item := range deliveries {
		view := CertificateDeliveryView{
			CertificateID:    item.CertificateID,
			Provider:         "deleted",
			CertSHA256:       item.CertSHA256,
			ReleaseSequence:  item.ReleaseSequence,
			DeliveryCount:    item.DeliveryCount,
			FirstDeliveredAt: item.FirstDeliveredAt,
			LastDeliveredAt:  item.LastDeliveredAt,
		}
		if cert, ok := certByID[item.CertificateID]; ok {
			currentSHA256 := edgeconfig.CertificatePEMDigest(cert.CertPEM)
			view.CertificateName = cert.Name
			view.PrimaryDomain = cert.PrimaryDomain
			view.Provider = cert.Provider
			view.ApplyStatus = cert.ApplyStatus
			if !cert.NotAfter.IsZero() {
				notAfter := cert.NotAfter
				view.NotAfter = &notAfter
			}
			view.CurrentCertSHA256 = currentSHA256
			view.MatchesCurrent = item.CertSHA256 != "" && item.CertSHA256 == currentSHA256
		}
		views = append(views, view)
	}
	return views, nil
}

// QuarantineNode revokes node credentials, disconnects WebSocket sessions, triggers DNS failover,
// and optionally rotates/revokes exposed ACME certificates.
func QuarantineNode(ctx context.Context, id uint, input QuarantineInput) (*QuarantineResult, error) {
	ctx = db.WithPrimaryReads(ctx)
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	oldToken := node.AccessToken
	newToken, err := newRandomToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	reason := truncateRunes(strings.TrimSpace(input.Reason), maxQuarantineReasonRunes)

	node.AccessToken = newToken
	node.QuarantinedAt = &now
	node.QuarantineReason = reason
	node.Status = nodeStatusQuarantined
	if err = repository.UpdateOpenFlareNodeFields(ctx, node, "access_token", "quarantined_at", "quarantine_reason", "status"); err != nil {
		return nil, err
	}

	listener.EmitNodeTokenRevoked(ctx, listener.NodeTokenRevoked{
		NodeID:   node.NodeID,
		OldToken: oldToken,
		Reason:   reason,
	})

	switchedGroups := make([]string, 0)
	unbackedGroups := make([]string, 0)
	if node.NodeType == "" || node.NodeType == nodeTypeEdgeNode {
		ofws.DisconnectAgent(ctx, node.NodeID, reason)
		var cfErr error
		switchedGroups, unbackedGroups, cfErr = cf.FailoverQuarantinedNode(ctx, node.ID)
		if cfErr != nil {
			logger.WarnF(ctx, "Cloudflare quarantine failover encountered errors for node %s: %v", node.NodeID, cfErr)
		}
	}

	deliveries, err := ListNodeCertificateDeliveries(ctx, node.ID)
	if err != nil {
		return nil, err
	}
	exposed := make([]CertificateDeliveryView, 0)
	seenCert := make(map[uint]struct{})
	rotatedIDs := make([]uint, 0)
	rotationErrors := make(map[uint]string)
	manualIDs := make([]uint, 0)

	for _, item := range deliveries {
		if !item.MatchesCurrent {
			continue
		}
		if _, dup := seenCert[item.CertificateID]; dup {
			continue
		}
		seenCert[item.CertificateID] = struct{}{}
		exposed = append(exposed, item)

		switch {
		case item.Provider == "acme" && input.RotateACMECerts:
			if _, rotErr := oftls.RotateCompromisedCertificate(ctx, item.CertificateID); rotErr != nil {
				rotationErrors[item.CertificateID] = rotErr.Error()
			} else {
				rotatedIDs = append(rotatedIDs, item.CertificateID)
			}
		case item.Provider != "acme" && item.Provider != "deleted":
			manualIDs = append(manualIDs, item.CertificateID)
		}
	}

	return &QuarantineResult{
		Node:                       buildNodeView(node),
		ExposedCertificates:        exposed,
		RotatedCertificateIDs:      rotatedIDs,
		RotationErrors:             rotationErrors,
		ManualActionCertificateIDs: manualIDs,
		DNSSwitchedGroups:          switchedGroups,
		DNSUnbackedGroups:          unbackedGroups,
	}, nil
}

// UnquarantineNode clears quarantine status and issues a fresh access token for a rebuilt node.
func UnquarantineNode(ctx context.Context, id uint) (*View, error) {
	ctx = db.WithPrimaryReads(ctx)
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	oldToken := node.AccessToken
	newToken, err := newRandomToken()
	if err != nil {
		return nil, err
	}

	changes := map[string]any{
		"access_token":      newToken,
		"quarantined_at":    nil,
		"quarantine_reason": "",
		"status":            nodeStatusPending,
		"last_seen_at":      nil,
	}
	if err = repository.UpdateOpenFlareNodeColumns(ctx, node, changes); err != nil {
		return nil, err
	}

	node.AccessToken = newToken
	node.QuarantinedAt = nil
	node.QuarantineReason = ""
	node.Status = nodeStatusPending
	node.LastSeenAt = nil

	listener.EmitNodeTokenRevoked(ctx, listener.NodeTokenRevoked{
		NodeID:   node.NodeID,
		OldToken: oldToken,
	})
	return buildNodeView(node), nil
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
