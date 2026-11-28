// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	embeddedInstanceID      = "embedded"
	instanceOnlineWindowSec = 45
)

// InstanceInput creates or updates a standalone SmartDNS worker instance.
type InstanceInput struct {
	Name         string   `json:"name"`
	PublicAddr   string   `json:"public_addr"`
	ListenAddr   string   `json:"listen_addr"`
	GeoName      string   `json:"geo_name"`
	GeoLatitude  *float64 `json:"geo_latitude"`
	GeoLongitude *float64 `json:"geo_longitude"`
}

// QuarantineInput contains an optional operator reason for isolating a SmartDNS instance.
type QuarantineInput struct {
	Reason string `json:"reason"`
}

// InstanceView exposes a registered SmartDNS instance to administrators.
type InstanceView struct {
	ID                    uint       `json:"id"`
	InstanceID            string     `json:"instance_id"`
	Name                  string     `json:"name"`
	Mode                  string     `json:"mode"`
	PublicAddr            string     `json:"public_addr"`
	ListenAddr            string     `json:"listen_addr"`
	GeoName               string     `json:"geo_name"`
	GeoLatitude           *float64   `json:"geo_latitude"`
	GeoLongitude          *float64   `json:"geo_longitude"`
	Version               string     `json:"version"`
	Status                string     `json:"status"`
	Connected             bool       `json:"connected"`
	SnapshotVersion       uint64     `json:"snapshot_version"`
	SnapshotETag          string     `json:"snapshot_etag"`
	QPS                   float64    `json:"qps"`
	P50Ms                 float64    `json:"p50_ms"`
	P95Ms                 float64    `json:"p95_ms"`
	P99Ms                 float64    `json:"p99_ms"`
	SuccessTotal          uint64     `json:"success_total"`
	ErrorTotal            uint64     `json:"error_total"`
	CacheHitRatio         float64    `json:"cache_hit_ratio"`
	CPUUsagePercent       float64    `json:"cpu_usage_percent"`
	MemoryUsagePercent    float64    `json:"memory_usage_percent"`
	RemoteUpdateSupported bool       `json:"remote_update_supported"`
	UpdateChannel         string     `json:"update_channel"`
	UpdateStatus          string     `json:"update_status"`
	UpdateRequestedAt     *time.Time `json:"update_requested_at"`
	LastSeenAt            *time.Time `json:"last_seen_at"`
	LastError             string     `json:"last_error"`
	QuarantinedAt         *time.Time `json:"quarantined_at"`
	QuarantineReason      string     `json:"quarantine_reason"`
	AccessToken           string     `json:"access_token,omitempty"`
	InstallCommand        string     `json:"install_command,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// DiscoveryTokenView returns the SmartDNS standalone instance discovery token.
type DiscoveryTokenView struct {
	DiscoveryToken string `json:"discovery_token"`
}

func toInstanceView(inst *model.OpenFlareDNSInstance, now time.Time, includeSecret bool) *InstanceView {
	if inst == nil {
		return nil
	}
	view := &InstanceView{
		ID:                    inst.ID,
		InstanceID:            inst.InstanceID,
		Name:                  inst.Name,
		Mode:                  inst.Mode,
		PublicAddr:            inst.PublicAddr,
		ListenAddr:            inst.ListenAddr,
		GeoName:               inst.GeoName,
		GeoLatitude:           inst.GeoLatitude,
		GeoLongitude:          inst.GeoLongitude,
		Version:               inst.Version,
		Status:                computeInstanceStatus(inst, now),
		Connected:             websocket.IsDNSInstanceConnected(inst.InstanceID),
		SnapshotVersion:       inst.SnapshotVersion,
		SnapshotETag:          inst.SnapshotETag,
		QPS:                   round4(inst.QPS),
		P50Ms:                 round4(inst.P50Ms),
		P95Ms:                 round4(inst.P95Ms),
		P99Ms:                 round4(inst.P99Ms),
		SuccessTotal:          inst.SuccessTotal,
		ErrorTotal:            inst.ErrorTotal,
		CacheHitRatio:         round4(inst.CacheHitRatio),
		CPUUsagePercent:       round4(inst.CPUUsagePercent),
		MemoryUsagePercent:    round4(inst.MemoryUsagePercent),
		RemoteUpdateSupported: inst.RemoteUpdateSupported,
		UpdateChannel:         inst.UpdateChannel,
		UpdateStatus:          instanceUpdateStatus(inst, now),
		UpdateRequestedAt:     inst.UpdateRequestedAt,
		LastSeenAt:            inst.LastSeenAt,
		LastError:             inst.LastError,
		QuarantinedAt:         inst.QuarantinedAt,
		QuarantineReason:      inst.QuarantineReason,
		CreatedAt:             inst.CreatedAt,
		UpdatedAt:             inst.UpdatedAt,
	}
	if includeSecret && inst.Mode == model.DNSInstanceModeStandalone {
		view.AccessToken = inst.AccessToken
		view.InstallCommand = buildInstallCommand(inst)
	}
	return view
}

func buildInstallCommand(inst *model.OpenFlareDNSInstance) string {
	if inst == nil || inst.AccessToken == "" {
		return ""
	}
	listen := strings.TrimSpace(inst.ListenAddr)
	if listen == "" {
		listen = defaultDNSListenAddr
	}
	return fmt.Sprintf(
		"curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && sudo bash install-openflare-dns.sh --controller-url <CONTROLLER_URL> --token-type instance --token %s --instance-id %s --listen %s",
		inst.AccessToken, inst.InstanceID, listen,
	)
}

func computeInstanceStatus(inst *model.OpenFlareDNSInstance, now time.Time) string {
	if inst == nil {
		return model.DNSInstanceStatusOffline
	}
	if inst.IsQuarantined() {
		return model.DNSInstanceStatusQuarantined
	}
	if inst.LastSeenAt == nil || inst.LastSeenAt.IsZero() {
		if inst.Status == "" {
			return model.DNSInstanceStatusPending
		}
		return inst.Status
	}
	if now.Sub(*inst.LastSeenAt) <= instanceOnlineWindowSec*time.Second {
		return model.DNSInstanceStatusOnline
	}
	return model.DNSInstanceStatusOffline
}

// ListInstances lists all embedded and standalone SmartDNS instances.
func ListInstances(ctx context.Context) ([]InstanceView, error) {
	rows, err := repository.ListDNSInstances(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]InstanceView, 0, len(rows))
	for i := range rows {
		out = append(out, *toInstanceView(&rows[i], now, false))
	}
	return out, nil
}

// CreateInstance pre-registers a standalone SmartDNS instance with its own instance_id and access_token.
func CreateInstance(ctx context.Context, input InstanceInput) (*InstanceView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 128 {
		return nil, newLogicError(http.StatusBadRequest, errInstanceNameRequired)
	}
	listenAddr := strings.TrimSpace(input.ListenAddr)
	if listenAddr == "" {
		listenAddr = defaultDNSListenAddr
	}
	suffix, err := generateHexToken(instanceIDRandomBytes)
	if err != nil {
		return nil, err
	}
	token, err := generateHexToken(instanceTokenBytes)
	if err != nil {
		return nil, err
	}
	item := &model.OpenFlareDNSInstance{
		InstanceID:   "dns-" + suffix,
		Name:         name,
		Mode:         model.DNSInstanceModeStandalone,
		PublicAddr:   strings.TrimSpace(input.PublicAddr),
		ListenAddr:   listenAddr,
		GeoName:      strings.TrimSpace(input.GeoName),
		GeoLatitude:  input.GeoLatitude,
		GeoLongitude: input.GeoLongitude,
		AccessToken:  token,
		Status:       model.DNSInstanceStatusPending,
	}
	if err := repository.CreateDNSInstance(ctx, item); err != nil {
		return nil, err
	}
	return toInstanceView(item, time.Now().UTC(), true), nil
}

// UpdateInstance updates metadata of an existing SmartDNS instance.
func UpdateInstance(ctx context.Context, id uint, input InstanceInput) (*InstanceView, error) {
	inst, err := repository.GetDNSInstanceByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newLogicError(http.StatusNotFound, errInstanceNotFound)
		}
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 128 {
		return nil, newLogicError(http.StatusBadRequest, errInstanceNameRequired)
	}
	inst.Name = name
	inst.PublicAddr = strings.TrimSpace(input.PublicAddr)
	if strings.TrimSpace(input.ListenAddr) != "" {
		inst.ListenAddr = strings.TrimSpace(input.ListenAddr)
	}
	inst.GeoName = strings.TrimSpace(input.GeoName)
	inst.GeoLatitude = input.GeoLatitude
	inst.GeoLongitude = input.GeoLongitude
	if err := repository.UpdateDNSInstanceFields(ctx, inst, "name", "public_addr", "listen_addr", "geo_name", "geo_latitude", "geo_longitude"); err != nil {
		return nil, err
	}
	return toInstanceView(inst, time.Now().UTC(), false), nil
}

// DeleteInstance removes a standalone SmartDNS instance (embedded instance is protected).
func DeleteInstance(ctx context.Context, id uint) error {
	inst, err := repository.GetDNSInstanceByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newLogicError(http.StatusNotFound, errInstanceNotFound)
		}
		return err
	}
	if inst.Mode == model.DNSInstanceModeEmbedded || inst.InstanceID == embeddedInstanceID {
		return newLogicError(http.StatusBadRequest, errInstanceEmbeddedLocked)
	}
	websocket.DisconnectDNSInstance(ctx, inst.InstanceID, "instance deleted")
	return repository.DeleteDNSInstance(ctx, id)
}

// QuarantineInstance isolates a SmartDNS instance and disconnects any live control WebSocket.
func QuarantineInstance(ctx context.Context, id uint, reason string) (*InstanceView, error) {
	inst, err := repository.GetDNSInstanceByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newLogicError(http.StatusNotFound, errInstanceNotFound)
		}
		return nil, err
	}
	now := time.Now().UTC()
	inst.QuarantinedAt = &now
	inst.QuarantineReason = strings.TrimSpace(reason)
	inst.Status = model.DNSInstanceStatusQuarantined
	if err := repository.UpdateDNSInstanceFields(ctx, inst, "quarantined_at", "quarantine_reason", colInstanceStatus); err != nil {
		return nil, err
	}
	websocket.DisconnectDNSInstance(ctx, inst.InstanceID, inst.QuarantineReason)
	return toInstanceView(inst, now, false), nil
}

// UnquarantineInstance clears quarantine on a SmartDNS instance.
func UnquarantineInstance(ctx context.Context, id uint) (*InstanceView, error) {
	inst, err := repository.GetDNSInstanceByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newLogicError(http.StatusNotFound, errInstanceNotFound)
		}
		return nil, err
	}
	inst.QuarantinedAt = nil
	inst.QuarantineReason = ""
	inst.LastSeenAt = nil
	inst.Status = model.DNSInstanceStatusOffline
	if err := repository.UpdateDNSInstanceFields(ctx, inst, "quarantined_at", "quarantine_reason", "last_seen_at", colInstanceStatus); err != nil {
		return nil, err
	}
	return toInstanceView(inst, time.Now().UTC(), false), nil
}

// RotateInstanceToken generates a fresh bearer token for a standalone SmartDNS instance.
func RotateInstanceToken(ctx context.Context, id uint) (*InstanceView, error) {
	inst, err := repository.GetDNSInstanceByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newLogicError(http.StatusNotFound, errInstanceNotFound)
		}
		return nil, err
	}
	token, err := generateHexToken(instanceTokenBytes)
	if err != nil {
		return nil, err
	}
	inst.AccessToken = token
	if err := repository.UpdateDNSInstanceFields(ctx, inst, "access_token"); err != nil {
		return nil, err
	}
	websocket.DisconnectDNSInstance(ctx, inst.InstanceID, "token rotated")
	return toInstanceView(inst, time.Now().UTC(), true), nil
}

// GetDiscoveryToken returns the global standalone DNS registration token.
func GetDiscoveryToken(ctx context.Context) (*DiscoveryTokenView, error) {
	tok, err := ensureSmartDNSDiscoveryToken(ctx)
	if err != nil {
		return nil, err
	}
	return &DiscoveryTokenView{DiscoveryToken: tok}, nil
}

// RotateDiscoveryToken rotates the global standalone DNS registration token.
func RotateDiscoveryToken(ctx context.Context) (*DiscoveryTokenView, error) {
	tok, err := generateHexToken(discoveryTokenBytes)
	if err != nil {
		return nil, err
	}
	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeySmartDNSDiscoveryToken, tok); err != nil {
		return nil, err
	}
	return &DiscoveryTokenView{DiscoveryToken: tok}, nil
}

// RegisterDNSInstance handles self-registration or credential refresh for standalone openflare-dns nodes.
func RegisterDNSInstance(ctx context.Context, existing *model.OpenFlareDNSInstance, req protocol.DNSInstanceRegisterRequest) (*protocol.DNSInstanceRegisterResponse, error) {
	if existing != nil {
		return updateRegisteredInstance(ctx, existing, req), nil
	}

	instanceID := strings.TrimSpace(req.InstanceID)
	if instanceID != "" {
		found, err := repository.GetDNSInstanceByInstanceID(ctx, instanceID)
		if err == nil && found != nil {
			if found.IsQuarantined() {
				return nil, newLogicError(http.StatusForbidden, errInstanceQuarantined)
			}
			return updateRegisteredInstance(ctx, found, req), nil
		}
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Standalone SmartDNS"
	}
	if instanceID == "" {
		suffix, err := generateHexToken(instanceIDRandomBytes)
		if err != nil {
			return nil, err
		}
		instanceID = "dns-" + suffix
	}
	token, err := generateHexToken(instanceTokenBytes)
	if err != nil {
		return nil, err
	}
	listen := strings.TrimSpace(req.ListenAddr)
	if listen == "" {
		listen = defaultDNSListenAddr
	}
	now := time.Now().UTC()
	item := &model.OpenFlareDNSInstance{
		InstanceID:   instanceID,
		Name:         name,
		Mode:         model.DNSInstanceModeStandalone,
		PublicAddr:   strings.TrimSpace(req.PublicAddr),
		ListenAddr:   listen,
		GeoName:      strings.TrimSpace(req.GeoName),
		GeoLatitude:  req.GeoLatitude,
		GeoLongitude: req.GeoLongitude,
		AccessToken:  token,
		Version:      strings.TrimSpace(req.Version),
		Status:       model.DNSInstanceStatusOnline,
		LastSeenAt:   &now,
	}
	if err := repository.CreateDNSInstance(ctx, item); err != nil {
		return nil, err
	}
	return &protocol.DNSInstanceRegisterResponse{
		ID:          item.ID,
		InstanceID:  item.InstanceID,
		Name:        item.Name,
		AccessToken: item.AccessToken,
	}, nil
}

func updateRegisteredInstance(
	ctx context.Context,
	existing *model.OpenFlareDNSInstance,
	req protocol.DNSInstanceRegisterRequest,
) *protocol.DNSInstanceRegisterResponse {
	updates := map[string]any{
		"version": strings.TrimSpace(req.Version),
	}
	if strings.TrimSpace(req.PublicAddr) != "" {
		updates["public_addr"] = strings.TrimSpace(req.PublicAddr)
	}
	if strings.TrimSpace(req.ListenAddr) != "" {
		updates["listen_addr"] = strings.TrimSpace(req.ListenAddr)
	}
	if strings.TrimSpace(req.GeoName) != "" {
		updates["geo_name"] = strings.TrimSpace(req.GeoName)
	}
	if req.GeoLatitude != nil {
		updates["geo_latitude"] = req.GeoLatitude
	}
	if req.GeoLongitude != nil {
		updates["geo_longitude"] = req.GeoLongitude
	}
	_ = repository.UpdateDNSInstanceRuntime(ctx, existing.InstanceID, updates)
	return &protocol.DNSInstanceRegisterResponse{
		ID:          existing.ID,
		InstanceID:  existing.InstanceID,
		Name:        existing.Name,
		AccessToken: existing.AccessToken,
	}
}

// RecordDNSHeartbeat updates instance runtime columns and enqueues an analytics DNSMetricSnapshot row.
func RecordDNSHeartbeat(ctx context.Context, inst *model.OpenFlareDNSInstance, req protocol.DNSHeartbeatRequest) (*protocol.DNSHeartbeatResponse, error) {
	if inst == nil {
		return nil, newLogicError(http.StatusUnauthorized, errTokenInvalid)
	}
	now := time.Now().UTC()
	capturedAt := now
	if req.CapturedAt != nil && !req.CapturedAt.IsZero() {
		capturedAt = req.CapturedAt.UTC()
	}

	updates := map[string]any{
		colInstanceStatus:         model.DNSInstanceStatusOnline,
		"snapshot_version":        req.SnapshotVersion,
		"snapshot_etag":           strings.TrimSpace(req.SnapshotETag),
		"qps":                     req.QPS,
		"p50_ms":                  req.P50Ms,
		"p95_ms":                  req.P95Ms,
		"p99_ms":                  req.P99Ms,
		"success_total":           req.QueriesSuccess,
		"error_total":             req.QueriesNXDomain + req.QueriesServFail + req.QueriesRateLimited,
		"cache_hit_ratio":         req.CacheHitRatio,
		"cpu_usage_percent":       req.CPUUsagePercent,
		"memory_usage_percent":    req.MemoryUsagePercent,
		"last_seen_at":            &now,
		colInstanceLastError:      strings.TrimSpace(req.LastError),
		"remote_update_supported": req.RemoteUpdateSupported,
	}
	if strings.TrimSpace(req.Version) != "" {
		updates["version"] = strings.TrimSpace(req.Version)
	}
	if strings.TrimSpace(req.PublicAddr) != "" {
		updates["public_addr"] = strings.TrimSpace(req.PublicAddr)
	}
	if strings.TrimSpace(req.ListenAddr) != "" {
		updates["listen_addr"] = strings.TrimSpace(req.ListenAddr)
	}
	if err := repository.UpdateDNSInstanceRuntime(ctx, inst.InstanceID, updates); err != nil {
		return nil, err
	}

	if err := repository.InsertOpenFlareDNSMetricSnapshot(ctx, &model.OpenFlareDNSMetricSnapshot{
		ID:                 idgen.NextUint64ID(),
		InstanceID:         inst.InstanceID,
		CapturedAt:         capturedAt,
		QueriesTotal:       req.QueriesTotal,
		QueriesSuccess:     req.QueriesSuccess,
		QueriesNXDomain:    req.QueriesNXDomain,
		QueriesServFail:    req.QueriesServFail,
		QueriesRateLimited: req.QueriesRateLimited,
		QPS:                req.QPS,
		LatencyP50Ms:       req.P50Ms,
		LatencyP95Ms:       req.P95Ms,
		LatencyP99Ms:       req.P99Ms,
		CacheHitRatio:      req.CacheHitRatio,
		SnapshotVersion:    req.SnapshotVersion,
		CPUUsagePercent:    req.CPUUsagePercent,
		MemoryUsagePercent: req.MemoryUsagePercent,
		CreatedAt:          now,
	}); err != nil {
		return nil, err
	}

	resp := &protocol.DNSHeartbeatResponse{OK: true}
	if bundle := DefaultSnapshotManager().Current(); bundle != nil {
		resp.SnapshotVersion = bundle.Envelope.Version
		resp.SnapshotETag = bundle.ETag
	}
	if err := attachDNSUpdate(ctx, inst, req, resp, now); err != nil {
		return nil, err
	}
	return resp, nil
}
