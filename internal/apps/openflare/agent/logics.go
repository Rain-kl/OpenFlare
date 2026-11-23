// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"

	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	"github.com/Rain-kl/Wavelet/internal/repository"

	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

// RegisterWithAccessToken registers an agent on a reserved node token.
func RegisterWithAccessToken(ctx context.Context, authNode *model.OpenFlareNode, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error) {
	_ = ofgeoip.EnsureRuntimeProvider(ctx)
	payload = normalizeNodePayload(payload)
	if authNode == nil {
		return nil, errors.New(errNodeNotFound)
	}
	if err := validateNodePayload(payload); err != nil {
		return nil, err
	}
	record, err := repository.GetOpenFlareNodeByAccessToken(ctx, authNode.AccessToken)
	if err != nil {
		return nil, err
	}
	if record.IsQuarantined() {
		return nil, errors.New(errNodeQuarantined)
	}
	if payload.Engine == enginePingora && record.NodeType != nodeTypeEdge {
		return nil, errors.New(errNodeEngineChanged)
	}
	previous := *record
	migrating := previous.LastSeenAt != nil && previous.Engine != payload.Engine
	if migrating {
		if previous.Engine != engineOpenResty || payload.Engine != enginePingora {
			return nil, errors.New(errNodeEngineChanged)
		}
		release, releaseErr := repository.GetActivePingoraConfigVersion(ctx)
		if releaseErr != nil && !isActiveConfigNotFound(releaseErr) {
			return nil, releaseErr
		}
		if release == nil || release.PayloadChecksum == "" {
			return nil, errors.New(errPingoraReleaseRequired)
		}
	}
	applyNodeRuntime(ctx, record, payload, true)
	if err = persistNodeRuntime(ctx, &previous, record); err != nil {
		return nil, err
	}
	return &protocol.RegisterNodeResponse{
		NodeID:      record.NodeID,
		AccessToken: record.AccessToken,
		Name:        record.Name,
	}, nil
}

func persistNodeRuntime(ctx context.Context, previous, current *model.OpenFlareNode) error {
	changes := collectHeartbeatChanges(previous, current)
	if len(changes) > 0 {
		err := repository.UpdateOpenFlareNodeRuntime(ctx, current, previous.Engine, changes)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errNodeEngineChanged)
		}
		if err != nil {
			return err
		}
	}
	RefreshAccessTokenCache(ctx, current)
	return nil
}

// RegisterWithDiscovery registers a new node using the global discovery token.
func RegisterWithDiscovery(ctx context.Context, payload protocol.NodePayload, registrationKey string) (*protocol.RegisterNodeResponse, error) {
	_ = ofgeoip.EnsureRuntimeProvider(ctx)
	payload = normalizeNodePayload(payload)
	if err := validateNodePayload(payload); err != nil {
		return nil, err
	}

	nodeID, accessToken, err := newDiscoveryIdentity(registrationKey)
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
		NodeType:         nodeTypeEdge,
		CapabilitiesJSON: "[]",
		UpdateChannel:    releaseChannelStable,
		Engine:           normalizeEngine(payload.Engine),
		ContractVersion:  payload.ContractVersion,
	}
	applyNodeRuntime(ctx, record, payload, false)

	if registrationKey != "" {
		record, err = repository.CreateOrGetOpenFlareDiscoveryNode(ctx, record)
		if err == nil && (record.NodeType != nodeTypeEdge || subtle.ConstantTimeCompare([]byte(record.AccessToken), []byte(accessToken)) != 1) {
			return nil, errors.New(errRegistrationCredentialChanged)
		}
	} else {
		err = repository.CreateOpenFlareNode(ctx, record)
	}
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errNodeIDConflict)
		}
		return nil, err
	}
	RefreshAccessTokenCache(ctx, record)
	return &protocol.RegisterNodeResponse{
		NodeID:      record.NodeID,
		AccessToken: record.AccessToken,
		Name:        record.Name,
	}, nil
}

func newDiscoveryIdentity(registrationKey string) (string, string, error) {
	if registrationKey != "" {
		return discoveryIdentity(registrationKey)
	}
	nodeID, err := newServerNodeID()
	if err != nil {
		return "", "", err
	}
	token, err := newRandomToken()
	return nodeID, token, err
}

// HeartbeatNode updates runtime state and returns agent settings.
func HeartbeatNode(ctx context.Context, authNode *model.OpenFlareNode, payload protocol.NodePayload) (*HeartbeatResponse, error) {
	_ = ofgeoip.EnsureRuntimeProvider(ctx)
	if authNode == nil {
		return nil, errors.New(errNodeNotFound)
	}
	payload.NodeID = authNode.NodeID
	payload = normalizeNodePayload(payload)
	if err := validateNodePayload(payload); err != nil {
		return nil, err
	}

	// Heartbeats renew the token cache. Load current settings from the database
	// so admin changes are visible across controller instances.
	authNode, err := LoadHeartbeatNode(ctx, authNode)
	if err != nil {
		return nil, err
	}
	if authNode.IsQuarantined() {
		return nil, errors.New(errNodeQuarantined)
	}
	if payload.Engine != normalizeEngine(authNode.Engine) {
		return nil, errors.New(errNodeEngineChanged)
	}
	previous := *authNode
	restartEdgeNow := authNode.RestartOpenrestyRequested
	applyNodeRuntime(ctx, authNode, payload, true)
	authNode.RestartOpenrestyRequested = false

	if err := persistNodeRuntime(ctx, &previous, authNode); err != nil {
		return nil, err
	}
	if previous.IP != authNode.IP && isPublicNodeIP(authNode.IP) {
		if _, dispatchErr := cf.DispatchNodeSync(ctx, authNode.ID, "cloudflare_agent_ip_update"); dispatchErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] enqueue heartbeat node sync failed: node_id=%d error=%v", authNode.ID, dispatchErr)
		}
	}

	reportedAt := time.Now()
	if authNode.LastSeenAt != nil {
		reportedAt = *authNode.LastSeenAt
	}
	PersistHeartbeatObservability(ctx, authNode.NodeID, payload, reportedAt)

	settings := buildAgentSettings(ctx, restartEdgeNow)
	updateTag := authNode.UpdateTag
	hasPendingUpdate := authNode.UpdateRequested && strings.HasPrefix(updateTag, "edge-v")
	if payload.EdgeUpdateSupported && payload.EdgeUpdateReady && hasPendingUpdate &&
		updateTag == "edge-v"+strings.TrimPrefix(payload.Version, "edge-v") {
		if err := repository.AcknowledgeOpenFlareEdgeUpdate(ctx, authNode); err != nil {
			return nil, err
		}
		authNode.UpdateRequested = false
		authNode.UpdateChannel = releaseChannelStable
		authNode.UpdateTag = ""
		hasPendingUpdate = false
	}
	if payload.EdgeUpdateSupported && (authNode.AutoUpdateEnabled || hasPendingUpdate) {
		settings.EdgeUpdate = &protocol.EdgeUpdateSettings{
			Repo:       node.AgentUpdateRepo(ctx),
			AutoUpdate: authNode.AutoUpdateEnabled,
		}
		if hasPendingUpdate {
			settings.EdgeUpdate.Tag = updateTag
		}
	}
	return &HeartbeatResponse{
		Config:        configManifest(ctx, authNode, payload.LatestConfigVersion),
		Node:          authNode,
		AgentSettings: settings,
	}, nil
}

// ReportApplyLog records an agent apply result.
func ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) (*model.OpenFlareApplyLog, error) {
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

	latest, err := repository.GetLatestOpenFlareApplyLogByNodeID(ctx, payload.NodeID)
	if err != nil {
		return nil, err
	}
	if payload.Engine != enginePingora && sameEngineApplyLog(latest, payload.Engine) &&
		model.IsRepeatSuccessApplyLog(latest, payload.Version, payload.Checksum, payload.Result) {
		if err := repository.UpdateOpenFlareNodeFromApplyResult(ctx, payload.NodeID, payload.Result, payload.Version, payload.Message, now); err != nil {
			return nil, err
		}
		return latest, nil
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
		Engine:              payload.Engine,
		ReleaseSequence:     payload.ReleaseSequence,
		PayloadChecksum:     payload.PayloadChecksum,
		CreatedAt:           now,
	}

	if err := repository.CreateOpenFlareApplyLogAndUpdateNode(ctx, log, payload.Result, payload.Version, payload.Message); err != nil {
		return nil, err
	}
	return log, nil
}

// ValidateDiscoveryToken delegates to the node package discovery token helper.
func ValidateDiscoveryToken(ctx context.Context, token string) error {
	return node.ValidateDiscoveryToken(ctx, token)
}
