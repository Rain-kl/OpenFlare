// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// PagesSourceTypeManual represents projects without a persisted source row.
	PagesSourceTypeManual = "manual"
	// PagesSourceTypeRemoteURL represents a persisted artifact URL.
	PagesSourceTypeRemoteURL = "remote_url"
	// PagesSourceTypeGitHubRelease is reserved for Phase 2.
	PagesSourceTypeGitHubRelease = "github_release"

	pagesSourceStatusIdle            = "idle"
	pagesSourceStatusChecking        = "checking"
	pagesSourceStatusUpdateAvailable = "update_available"
	pagesSourceStatusSyncing         = "syncing"
	pagesSourceStatusFailed          = "failed"
	pagesSourceStatusAttention       = "attention"

	defaultRemoteAssetLabel = "pages-package"
)

// SourceUpdateInput is the discriminated source configuration payload.
// GitHub fields are accepted by the decoder so mode-incompatible values can be
// rejected deterministically; GitHub itself is enabled in Phase 2.
type SourceUpdateInput struct {
	SourceType           string `json:"source_type"`
	RemoteURLSet         bool   `json:"remote_url_set"`
	RemoteURL            string `json:"remote_url"`
	RemoteNetworkPolicy  string `json:"remote_network_policy"`
	RepositoryURL        string `json:"repository_url"`
	ReleaseSelector      string `json:"release_selector"`
	ReleaseTag           string `json:"release_tag"`
	AssetName            string `json:"asset_name"`
	AutoUpdateEnabled    bool   `json:"auto_update_enabled"`
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
}

// SourceRevisionView is a credential-free source cursor.
type SourceRevisionView struct {
	Revision  string `json:"revision"`
	Label     string `json:"label"`
	AssetName string `json:"asset_name,omitempty"`
}

// SourceView is the safe discriminated source view returned to the console.
type SourceView struct {
	SourceType           string              `json:"source_type"`
	HasRemoteURL         bool                `json:"has_remote_url,omitempty"`
	DisplayURL           string              `json:"display_url,omitempty"`
	RemoteNetworkPolicy  string              `json:"remote_network_policy,omitempty"`
	GitHubRepository     string              `json:"github_repository,omitempty"`
	ReleaseSelector      string              `json:"release_selector,omitempty"`
	ReleaseTag           string              `json:"release_tag,omitempty"`
	AssetName            string              `json:"asset_name,omitempty"`
	AutoUpdateEnabled    bool                `json:"auto_update_enabled,omitempty"`
	CheckIntervalMinutes int                 `json:"check_interval_minutes,omitempty"`
	SyncStatus           string              `json:"sync_status,omitempty"`
	UpdateAvailable      bool                `json:"update_available,omitempty"`
	LastSeen             *SourceRevisionView `json:"last_seen,omitempty"`
	LastApplied          *SourceRevisionView `json:"last_applied,omitempty"`
	LastCheckedAt        *time.Time          `json:"last_checked_at,omitempty"`
	LastSyncedAt         *time.Time          `json:"last_synced_at,omitempty"`
	NextCheckAt          *time.Time          `json:"next_check_at,omitempty"`
	LastError            string              `json:"last_error,omitempty"`
}

// SourceActionReceipt identifies the internal task execution created by an action API.
type SourceActionReceipt struct {
	TaskID      string `json:"task_id"`
	ExecutionID string `json:"execution_id"`
	Action      string `json:"action"`
}

// SourceUpdateResult is returned after persisting source configuration.
type SourceUpdateResult struct {
	Source    *SourceView          `json:"source"`
	CheckTask *SourceActionReceipt `json:"check_task"`
	Warning   string               `json:"warning"`
}

type sourceDetail struct {
	Provider  string `json:"provider"`
	Label     string `json:"label"`
	AssetName string `json:"asset_name,omitempty"`
	ReleaseID string `json:"release_id,omitempty"`
}

type remoteSourceConfig struct {
	URL      string
	Policy   string
	Identity string
}

// GetSource returns the current persisted source or a manual discriminator.
func GetSource(ctx context.Context, projectID uint) (*SourceView, error) {
	if _, err := model.GetPagesProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	source, runtime, err := loadSourceByProject(ctx, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &SourceView{SourceType: PagesSourceTypeManual}, nil
	}
	if err != nil {
		return nil, err
	}
	return buildSourceView(source, runtime)
}

// UpdateSource creates or updates a Remote URL source and its 1:1 runtime row.
func UpdateSource(ctx context.Context, projectID uint, input SourceUpdateInput) (*SourceUpdateResult, error) {
	if err := validateRemoteSourceInput(input); err != nil {
		return nil, err
	}

	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		return updateRemoteSourceTx(tx, projectID, input)
	})
	if err != nil {
		return nil, err
	}

	view, err := GetSource(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &SourceUpdateResult{Source: view, Warning: ""}, nil
}

func updateRemoteSourceTx(tx *gorm.DB, projectID uint, input SourceUpdateInput) error {
	var project model.PagesProject
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, projectID).Error; err != nil {
		return err
	}
	existing, hasExisting, err := loadProjectSourceForUpdate(tx, projectID)
	if err != nil {
		return err
	}
	config, err := buildRemoteSourceConfig(existing, hasExisting, input)
	if err != nil {
		return err
	}
	if !hasExisting {
		return createRemoteSourceTx(tx, projectID, config)
	}
	return updateExistingRemoteSourceTx(tx, existing, config)
}

func loadProjectSourceForUpdate(tx *gorm.DB, projectID uint) (*model.PagesProjectSource, bool, error) {
	var source model.PagesProjectSource
	err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("project_id = ?", projectID).
		First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &source, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &source, true, nil
}

func buildRemoteSourceConfig(
	existing *model.PagesProjectSource,
	hasExisting bool,
	input SourceUpdateInput,
) (remoteSourceConfig, error) {
	remoteURL, err := resolveUpdatedRemoteURL(existing, hasExisting, input)
	if err != nil {
		return remoteSourceConfig{}, err
	}
	parsedURL, err := parseRemoteSourceURL(remoteURL)
	if err != nil {
		return remoteSourceConfig{}, err
	}
	policy := strings.TrimSpace(input.RemoteNetworkPolicy)
	if policy == "" {
		policy = RemoteNetworkPolicyPublic
	}
	return remoteSourceConfig{
		URL:      remoteURL,
		Policy:   policy,
		Identity: remoteSourceIdentity(parsedURL),
	}, nil
}

func createRemoteSourceTx(tx *gorm.DB, projectID uint, config remoteSourceConfig) error {
	source := &model.PagesProjectSource{
		ProjectID:            projectID,
		SourceType:           PagesSourceTypeRemoteURL,
		RemoteURL:            config.URL,
		RemoteNetworkPolicy:  config.Policy,
		AutoUpdateEnabled:    false,
		CheckIntervalMinutes: 0,
		ConfigVersion:        1,
		SourceIdentity:       config.Identity,
	}
	if err := tx.Create(source).Error; err != nil {
		return err
	}
	return tx.Create(&model.PagesProjectSourceRuntime{
		SourceID:   source.ID,
		SyncStatus: pagesSourceStatusIdle,
	}).Error
}

func updateExistingRemoteSourceTx(
	tx *gorm.DB,
	existing *model.PagesProjectSource,
	config remoteSourceConfig,
) error {
	if !remoteSourceConfigChanged(existing, config) {
		return nil
	}
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", existing.ID).
		First(&runtime).Error; err != nil {
		return err
	}
	identityChanged := existing.SourceIdentity != config.Identity
	if err := tx.Model(existing).Updates(map[string]any{
		"source_type":            PagesSourceTypeRemoteURL,
		"remote_url":             config.URL,
		"remote_network_policy":  config.Policy,
		"github_repository":      "",
		"release_selector":       "",
		"release_tag":            "",
		"asset_name":             "",
		"auto_update_enabled":    false,
		"check_interval_minutes": 0,
		"config_version":         existing.ConfigVersion + 1,
		"source_identity":        config.Identity,
	}).Error; err != nil {
		return err
	}
	return resetRuntimeAfterSourceUpdate(tx, &runtime, identityChanged)
}

func remoteSourceConfigChanged(existing *model.PagesProjectSource, config remoteSourceConfig) bool {
	return existing.SourceType != PagesSourceTypeRemoteURL ||
		existing.RemoteURL != config.URL ||
		existing.RemoteNetworkPolicy != config.Policy ||
		existing.GitHubRepository != "" ||
		existing.ReleaseSelector != "" ||
		existing.ReleaseTag != "" ||
		existing.AssetName != "" ||
		existing.AutoUpdateEnabled ||
		existing.CheckIntervalMinutes != 0
}

// DeleteSource idempotently switches a project back to manual mode.
func DeleteSource(ctx context.Context, projectID uint) (*SourceView, error) {
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, projectID).Error; err != nil {
			return err
		}
		var source model.PagesProjectSource
		err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			Where("project_id = ?", projectID).
			First(&source).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var runtime model.PagesProjectSourceRuntime
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			Where("source_id = ?", source.ID).
			First(&runtime).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Where("source_id = ?", source.ID).Delete(&model.PagesProjectSourceRuntime{}).Error; err != nil {
			return err
		}
		return tx.Delete(&source).Error
	})
	if err != nil {
		return nil, err
	}
	return &SourceView{SourceType: PagesSourceTypeManual}, nil
}

func validateRemoteSourceInput(input SourceUpdateInput) error {
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType == "" {
		return errors.New(errPagesSourceTypeRequired)
	}
	if sourceType != PagesSourceTypeRemoteURL {
		return errors.New(errPagesSourceTypeUnsupported)
	}
	if strings.TrimSpace(input.RepositoryURL) != "" || strings.TrimSpace(input.ReleaseSelector) != "" ||
		strings.TrimSpace(input.ReleaseTag) != "" || strings.TrimSpace(input.AssetName) != "" ||
		input.AutoUpdateEnabled || input.CheckIntervalMinutes != 0 {
		return errors.New(errPagesSourceRemoteFields)
	}
	policy := strings.TrimSpace(input.RemoteNetworkPolicy)
	if policy != "" && policy != RemoteNetworkPolicyPublic && policy != RemoteNetworkPolicyTrustedInternal {
		return errors.New(errPagesSourceNetworkPolicy)
	}
	if !input.RemoteURLSet && strings.TrimSpace(input.RemoteURL) != "" {
		return errors.New(errPagesSourceRemoteURLMode)
	}
	if input.RemoteURLSet && strings.TrimSpace(input.RemoteURL) == "" {
		return errors.New(errPagesSourceRemoteURLRequired)
	}
	return nil
}

func resolveUpdatedRemoteURL(existing *model.PagesProjectSource, hasExisting bool, input SourceUpdateInput) (string, error) {
	if input.RemoteURLSet {
		return strings.TrimSpace(input.RemoteURL), nil
	}
	if !hasExisting || existing.SourceType != PagesSourceTypeRemoteURL || strings.TrimSpace(existing.RemoteURL) == "" {
		return "", errors.New(errPagesSourceRemoteURLRequired)
	}
	return existing.RemoteURL, nil
}

func parseRemoteSourceURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New(errPagesSourceRemoteURLInvalid)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != remoteSourceSchemeHTTP && parsed.Scheme != remoteSourceSchemeHTTPS {
		return nil, errors.New(errPagesSourceRemoteURLInvalid)
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, errors.New(errPagesSourceRemoteURLInvalid)
	}
	return parsed, nil
}

func remoteSourceIdentity(parsed *url.URL) string {
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		port = ""
	}
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	canonicalPath := parsed.EscapedPath()
	if canonicalPath == "" {
		canonicalPath = "/"
	}
	canonicalPath = path.Clean("/" + strings.TrimPrefix(canonicalPath, "/"))
	canonical := parsed.Scheme + "://" + host + canonicalPath
	sum := sha256.Sum256([]byte("remote_url|" + canonical))
	return hex.EncodeToString(sum[:])
}

func displayRemoteSourceURL(raw string) string {
	parsed, err := parseRemoteSourceURL(raw)
	if err != nil {
		return ""
	}
	hadQuery := parsed.RawQuery != ""
	parsed.RawQuery = ""
	display := parsed.String()
	if hadQuery {
		display += "?***"
	}
	return display
}

func loadSourceByProject(ctx context.Context, projectID uint) (*model.PagesProjectSource, *model.PagesProjectSourceRuntime, error) {
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("project_id = ?", projectID).First(&source).Error; err != nil {
		return nil, nil, err
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&runtime).Error; err != nil {
		return nil, nil, err
	}
	return &source, &runtime, nil
}

func buildSourceView(source *model.PagesProjectSource, runtime *model.PagesProjectSourceRuntime) (*SourceView, error) {
	if source == nil || runtime == nil {
		return nil, errors.New(errPagesSourceNotFound)
	}
	view := &SourceView{
		SourceType:   source.SourceType,
		SyncStatus:   runtime.SyncStatus,
		LastSyncedAt: runtime.LastSyncedAt,
		LastError:    runtime.LastError,
	}
	if runtime.LastAppliedRevision != "" {
		view.LastApplied = revisionView(runtime.LastAppliedRevision, runtime.LastAppliedDetail)
	}
	switch source.SourceType {
	case PagesSourceTypeRemoteURL:
		view.HasRemoteURL = strings.TrimSpace(source.RemoteURL) != ""
		view.DisplayURL = displayRemoteSourceURL(source.RemoteURL)
		view.RemoteNetworkPolicy = source.RemoteNetworkPolicy
	case PagesSourceTypeGitHubRelease:
		view.LastCheckedAt = runtime.LastCheckedAt
		view.NextCheckAt = runtime.NextCheckAt
		view.UpdateAvailable = runtime.LastSeenRevision != "" && runtime.LastSeenRevision != runtime.LastAppliedRevision
		if runtime.LastSeenRevision != "" {
			view.LastSeen = revisionView(runtime.LastSeenRevision, runtime.LastSeenDetail)
		}
		view.GitHubRepository = source.GitHubRepository
		view.ReleaseSelector = source.ReleaseSelector
		view.ReleaseTag = source.ReleaseTag
		view.AssetName = source.AssetName
		view.AutoUpdateEnabled = source.AutoUpdateEnabled
		view.CheckIntervalMinutes = source.CheckIntervalMinutes
	default:
		return nil, errors.New(errPagesSourceTypeUnsupported)
	}
	return view, nil
}

func revisionView(revision string, detailJSON string) *SourceRevisionView {
	detail := sourceDetail{}
	_ = unmarshalSourceDetail(detailJSON, &detail)
	label := strings.TrimSpace(detail.Label)
	if label == "" {
		label = defaultRemoteAssetLabel
	}
	return &SourceRevisionView{
		Revision:  revision,
		Label:     label,
		AssetName: detail.AssetName,
	}
}

func unmarshalSourceDetail(raw string, detail *sourceDetail) error {
	if detail == nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), detail)
}

func resetRuntimeAfterSourceUpdate(tx *gorm.DB, runtime *model.PagesProjectSourceRuntime, identityChanged bool) error {
	updates := map[string]any{
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
		sourceRuntimeColumnLastError:      "",
	}
	if identityChanged {
		updates["etag"] = ""
		updates["last_seen_revision"] = ""
		updates["last_seen_detail"] = ""
		updates["last_applied_revision"] = ""
		updates["last_applied_detail"] = ""
		updates["last_checked_at"] = nil
		updates["last_synced_at"] = nil
		updates["next_check_at"] = nil
		updates[sourceRuntimeColumnSyncStatus] = pagesSourceStatusIdle
	} else {
		updates[sourceRuntimeColumnSyncStatus] = normalizedSourceRuntimeStatus(runtime)
	}
	return tx.Model(runtime).Updates(updates).Error
}

func normalizedSourceRuntimeStatus(runtime *model.PagesProjectSourceRuntime) string {
	if runtime == nil {
		return pagesSourceStatusIdle
	}
	if sourceHasSameReleaseReplacement(runtime) {
		return pagesSourceStatusAttention
	}
	if runtime.LastSeenRevision != "" && runtime.LastSeenRevision != runtime.LastAppliedRevision {
		return pagesSourceStatusUpdateAvailable
	}
	return pagesSourceStatusIdle
}
