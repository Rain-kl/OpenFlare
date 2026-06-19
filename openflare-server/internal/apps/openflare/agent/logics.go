// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// RegisterWithAccessToken registers an agent on a reserved node token.
func RegisterWithAccessToken(ctx context.Context, authNode *model.OpenFlareNode, payload NodePayload) (*RegistrationResponse, error) {
	payload = normalizeNodePayload(payload)
	if authNode == nil {
		return nil, errors.New(errNodeNotFound)
	}
	if err := validateNodePayload(payload); err != nil {
		return nil, err
	}
	applyNodeRuntime(authNode, payload, true)
	if err := model.SaveOpenFlareNode(ctx, authNode); err != nil {
		return nil, err
	}
	RefreshAccessTokenCache(ctx, authNode)
	return &RegistrationResponse{
		NodeID:      authNode.NodeID,
		AccessToken: authNode.AccessToken,
		Name:        authNode.Name,
	}, nil
}

// RegisterWithDiscovery registers a new node using the global discovery token.
func RegisterWithDiscovery(ctx context.Context, payload NodePayload) (*RegistrationResponse, error) {
	payload = normalizeNodePayload(payload)
	if err := validateNodePayload(payload); err != nil {
		return nil, err
	}

	nodeID, err := newServerNodeID()
	if err != nil {
		return nil, err
	}
	accessToken, err := newRandomToken()
	if err != nil {
		return nil, err
	}

	nodeName := payload.Name
	if nodeName == "" {
		nodeName = nodeID
	}

	record := &model.OpenFlareNode{
		NodeID:           nodeID,
		Name:             nodeName,
		AccessToken:      accessToken,
		Status:           nodeStatusOnline,
		NodeType:         "edge_node",
		CapabilitiesJSON: "[]",
		UpdateChannel:    releaseChannelStable,
	}
	applyNodeRuntime(record, payload, false)

	if err = model.CreateOpenFlareNode(ctx, record); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errNodeIDConflict)
		}
		return nil, err
	}
	RefreshAccessTokenCache(ctx, record)
	return &RegistrationResponse{
		NodeID:      record.NodeID,
		AccessToken: record.AccessToken,
		Name:        record.Name,
	}, nil
}

// HeartbeatNode updates runtime state and returns agent settings.
func HeartbeatNode(ctx context.Context, authNode *model.OpenFlareNode, payload NodePayload) (*HeartbeatResponse, error) {
	if authNode == nil {
		return nil, errors.New(errNodeNotFound)
	}
	payload.NodeID = authNode.NodeID
	payload = normalizeNodePayload(payload)
	if err := validateNodePayload(payload); err != nil {
		return nil, err
	}

	previous := *authNode
	updateNow := authNode.UpdateRequested
	restartOpenrestyNow := authNode.RestartOpenrestyRequested
	updateChannel := strings.TrimSpace(authNode.UpdateChannel)
	updateTag := strings.TrimSpace(authNode.UpdateTag)

	applyNodeRuntime(authNode, payload, true)
	authNode.UpdateRequested = false
	authNode.UpdateChannel = releaseChannelStable
	authNode.UpdateTag = ""
	authNode.RestartOpenrestyRequested = false

	changes := collectHeartbeatChanges(&previous, authNode)
	if len(changes) > 0 {
		fields := make([]string, 0, len(changes))
		for field := range changes {
			fields = append(fields, field)
		}
		if err := model.UpdateOpenFlareNodeFields(ctx, authNode, fields...); err != nil {
			return nil, err
		}
	}

	RefreshAccessTokenCache(ctx, authNode)

	reportedAt := time.Now()
	if authNode.LastSeenAt != nil {
		reportedAt = *authNode.LastSeenAt
	}
	PersistHeartbeatObservability(ctx, authNode.NodeID, payload, reportedAt)

	activeConfig, err := getActiveConfigMeta(ctx)
	if err != nil && !isActiveConfigNotFound(err) {
		return nil, err
	}

	wafIPGroups, err := ChangedWAFIPGroupsForAgent(ctx, nil, payload.WAFIPGroupChecksums)
	if err != nil {
		return nil, err
	}

	return &HeartbeatResponse{
		Node:          authNode,
		AgentSettings: buildAgentSettings(authNode, updateNow, updateChannel, updateTag, restartOpenrestyNow),
		ActiveConfig:  activeConfig,
		WAFIPGroups:   wafIPGroups,
	}, nil
}

// GetActiveConfig returns the active configuration for an agent.
func GetActiveConfig(ctx context.Context) (*ConfigResponse, error) {
	config, err := getActiveConfigForAgent(ctx)
	if err != nil {
		if isActiveConfigNotFound(err) {
			return nil, errors.New(errNoActiveConfig)
		}
		return nil, err
	}
	return config, nil
}

// SyncWAFIPGroups returns WAF IP groups whose checksums differ from the agent state.
func SyncWAFIPGroups(ctx context.Context, input WAFIPGroupSyncInput) (*WAFIPGroupSyncResult, error) {
	groups, err := ChangedWAFIPGroupsForAgent(ctx, input.IDs, input.Checksums)
	if err != nil {
		return nil, err
	}
	return &WAFIPGroupSyncResult{Groups: groups}, nil
}

// ReportApplyLog records an agent apply result.
func ReportApplyLog(ctx context.Context, payload ApplyLogPayload) (*model.OpenFlareApplyLog, error) {
	now := time.Now()
	payload = normalizeApplyLogPayload(payload)
	if payload.NodeID == "" {
		return nil, errors.New(errNodeIDRequired)
	}
	if payload.Version == "" {
		return nil, errors.New(errVersionRequired)
	}
	if payload.Result != applyResultOK && payload.Result != applyResultWarn && payload.Result != applyResultFailed {
		return nil, errors.New(errInvalidApplyResult)
	}

	log := &model.OpenFlareApplyLog{
		NodeID:              payload.NodeID,
		Version:             payload.Version,
		Result:              payload.Result,
		Message:             payload.Message,
		Checksum:            payload.Checksum,
		MainConfigChecksum:  payload.MainConfigChecksum,
		RouteConfigChecksum: payload.RouteConfigChecksum,
		SupportFileCount:    payload.SupportFileCount,
		CreatedAt:           now,
	}

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New("database not initialized")
	}

	err := conn.Transaction(func(tx *gorm.DB) error {
		record := &model.OpenFlareNode{}
		if err := tx.Where("node_id = ?", payload.NodeID).First(record).Error; err != nil {
			return err
		}
		record.Status = nodeStatusOnline
		record.LastSeenAt = &now
		if payload.Result == applyResultOK {
			record.CurrentVersion = payload.Version
			record.LastError = ""
		} else {
			record.LastError = payload.Message
		}
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return tx.Model(record).Select("status", "last_seen_at", "current_version", "last_error").Updates(record).Error
	})
	if err != nil {
		return nil, err
	}
	return log, nil
}

// ValidateDiscoveryToken delegates to the node package discovery token helper.
func ValidateDiscoveryToken(ctx context.Context, token string) error {
	return node.ValidateDiscoveryToken(ctx, token)
}
