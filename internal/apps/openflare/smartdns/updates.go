// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	dnsUpdateExpiry        = 20 * time.Minute
	errUpdateSelection     = "select at least one SmartDNS instance and a valid release channel"
	errUpdateUnavailable   = "all selected instances must be online, support remote updates and have no update in progress"
	errUpdateRequestFailed = "failed to request SmartDNS instance updates"
)

// InstanceUpdateInput requests one release channel for the selected standalone instances.
type InstanceUpdateInput struct {
	InstanceIDs []uint `json:"instance_ids"`
	Channel     string `json:"channel" enums:"stable,canary"`
}

// InstanceUpdateResult reports the number of instances queued for update.
type InstanceUpdateResult struct {
	Queued int `json:"queued"`
}

// RequestInstanceUpdates queues a release channel for the selected instances.
func RequestInstanceUpdates(ctx context.Context, input InstanceUpdateInput) (*InstanceUpdateResult, error) {
	if (input.Channel != "stable" && input.Channel != "canary") || len(input.InstanceIDs) == 0 || len(input.InstanceIDs) > 500 {
		return nil, newLogicError(http.StatusBadRequest, errUpdateSelection)
	}
	ids := make([]uint, 0, len(input.InstanceIDs))
	seen := make(map[uint]bool, len(input.InstanceIDs))
	for _, id := range input.InstanceIDs {
		if id == 0 {
			return nil, newLogicError(http.StatusBadRequest, errUpdateSelection)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	now := time.Now().UTC()
	err := repository.QueueDNSInstanceUpdates(ctx, ids, rand.Text(), input.Channel, now, now.Add(-instanceOnlineWindowSec*time.Second), now.Add(-dnsUpdateExpiry))
	if errors.Is(err, repository.ErrDNSUpdateUnavailable) {
		return nil, newLogicError(http.StatusConflict, errUpdateUnavailable)
	}
	if err != nil {
		return nil, err
	}
	return &InstanceUpdateResult{Queued: len(ids)}, nil
}

func instanceUpdateStatus(inst *model.OpenFlareDNSInstance, now time.Time) string {
	if (inst.UpdateStatus == model.DNSUpdateQueued || inst.UpdateStatus == model.DNSUpdateRunning) &&
		inst.UpdateRequestedAt != nil && !now.Before(inst.UpdateRequestedAt.Add(dnsUpdateExpiry)) {
		return model.DNSUpdateTimedOut
	}
	return inst.UpdateStatus
}

func attachDNSUpdate(ctx context.Context, inst *model.OpenFlareDNSInstance, req protocol.DNSHeartbeatRequest, response *protocol.DNSHeartbeatResponse, now time.Time) error {
	if inst.Mode != model.DNSInstanceModeStandalone || !req.RemoteUpdateSupported || inst.UpdateRequestID == "" {
		return nil
	}
	updateID := inst.UpdateRequestID
	if receipt := req.UpdateReceipt; receipt != nil && receipt.ID == updateID {
		switch receipt.Status {
		case model.DNSUpdateRunning, model.DNSUpdateSucceeded, model.DNSUpdateFailed:
			return repository.AcknowledgeDNSInstanceUpdate(ctx, inst.InstanceID, updateID, receipt.Status)
		}
	}
	if instanceUpdateStatus(inst, now) == model.DNSUpdateQueued {
		response.Update = &protocol.DNSUpdateCommand{ID: updateID, Channel: inst.UpdateChannel}
	}
	return nil
}
