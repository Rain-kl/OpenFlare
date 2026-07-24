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

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	// PagesSourceTypeManual represents projects without a persisted source row.
	PagesSourceTypeManual = "manual"
	// PagesSourceTypeRemoteURL represents a persisted artifact URL.
	PagesSourceTypeRemoteURL = "remote_url"
	// PagesSourceTypeGitHubRelease represents a public GitHub Release asset.
	PagesSourceTypeGitHubRelease = "github_release"

	pagesSourceStatusIdle            = "idle"
	pagesSourceStatusChecking        = "checking"
	pagesSourceStatusUpdateAvailable = "update_available"
	pagesSourceStatusSyncing         = "syncing"
	pagesSourceStatusFailed          = "failed"
	pagesSourceStatusAttention       = "attention"

	defaultRemoteAssetLabel = "pages-package"
	defaultGitHubAssetName  = "dist.zip"
	defaultCheckInterval    = 1440
	minimumCheckInterval    = 5
	maximumCheckInterval    = 1440
)

// SourceUpdateInput is the discriminated source configuration payload.
// GitHub fields are accepted by the decoder so mode-incompatible values can be
// rejected deterministically.
type SourceUpdateInput struct {
	SourceType           string `json:"source_type"`
	RemoteURL            string `json:"remote_url"`
	AllowInsecure        bool   `json:"allow_insecure"`
	RepositoryURL        string `json:"repository_url"`
	ReleaseSelector      string `json:"release_selector"`
	ReleaseTag           string `json:"release_tag"`
	AssetName            string `json:"asset_name"`
	AutoUpdateEnabled    bool   `json:"auto_update_enabled"`
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
}

// SourceRevisionView is a source cursor shown to the console.
type SourceRevisionView struct {
	Revision  string `json:"revision"`
	Label     string `json:"label"`
	AssetName string `json:"asset_name,omitempty"`
}

// SourceView is the discriminated source view returned to the console.
type SourceView struct {
	SourceType           string              `json:"source_type"`
	RemoteURL            string              `json:"remote_url,omitempty"`
	AllowInsecure        bool                `json:"allow_insecure,omitempty"`
	GitHubRepository     string              `json:"github_repository,omitempty"`
	ReleaseSelector      string              `json:"release_selector,omitempty"`
	ReleaseTag           string              `json:"release_tag,omitempty"`
	AssetName            string              `json:"asset_name,omitempty"`
	AutoUpdateEnabled    *bool               `json:"auto_update_enabled,omitempty"`
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
	Provider       string `json:"provider"`
	DisplayName    string `json:"display_name,omitempty"`
	Tag            string `json:"tag,omitempty"`
	LegacyLabel    string `json:"label,omitempty"`
	AssetName      string `json:"asset_name,omitempty"`
	ReleaseID      string `json:"release_id,omitempty"`
	AssetID        string `json:"asset_id,omitempty"`
	AssetUpdatedAt string `json:"asset_updated_at,omitempty"`
	Digest         string `json:"digest,omitempty"`
}

type remoteSourceConfig struct {
	URL           string
	AllowInsecure bool
	Identity      string
}

// GetSource returns the current persisted source or a manual discriminator.
func GetSource(ctx context.Context, projectID uint) (*SourceView, error) {
	if _, err := repository.GetPagesProjectByID(ctx, projectID); err != nil {
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

// UpdateSource creates or updates a source. Direct callers use the system actor;
// HTTP handlers should call UpdateSourceAs so the initial check is auditable.
func UpdateSource(ctx context.Context, projectID uint, input SourceUpdateInput) (*SourceUpdateResult, error) {
	return UpdateSourceAs(ctx, projectID, input, pagesSourceCreatedBySystem)
}

// UpdateSourceAs persists source configuration and queues the first GitHub check
// after commit when the GitHub configuration was materially changed.
func UpdateSourceAs(
	ctx context.Context,
	projectID uint,
	input SourceUpdateInput,
	actor string,
) (*SourceUpdateResult, error) {
	if !validPagesSourceActor(actor) {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	if err := validateSourceUpdateInput(input); err != nil {
		return nil, err
	}

	changed := false
	var persistedSource model.PagesProjectSource
	err := repository.WithPagesTx(ctx, func(tx *gorm.DB) error {
		var err error
		switch strings.TrimSpace(input.SourceType) {
		case PagesSourceTypeRemoteURL:
			changed, err = updateRemoteSourceTx(tx, projectID, input)
		case PagesSourceTypeGitHubRelease:
			changed, err = updateGitHubSourceTx(tx, projectID, input)
		default:
			err = errors.New(errPagesSourceTypeUnsupported)
		}
		if err != nil || !changed || strings.TrimSpace(input.SourceType) != PagesSourceTypeGitHubRelease {
			return err
		}
		source, loadErr := repository.LockPagesProjectSourceByProjectIDTx(tx, projectID)
		if loadErr != nil {
			return loadErr
		}
		persistedSource = *source
		return nil
	})
	if err != nil {
		return nil, err
	}

	view, err := GetSource(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := &SourceUpdateResult{Source: view, Warning: ""}
	if changed && strings.TrimSpace(input.SourceType) == PagesSourceTypeGitHubRelease {
		receipt, dispatchErr := dispatchSourceActionSnapshot(ctx, persistedSource, sourceActionCheck, actor, "", "", "manual")
		if dispatchErr != nil {
			result.Warning = errPagesSourceInitialCheckWarning
			markInitialCheckDispatchFailed(ctx, persistedSource.ID, persistedSource.ConfigVersion)
		} else {
			result.CheckTask = receipt
		}
	}
	return result, nil
}

func updateRemoteSourceTx(tx *gorm.DB, projectID uint, input SourceUpdateInput) (bool, error) {
	if _, err := repository.LockPagesProjectByIDTx(tx, projectID); err != nil {
		return false, err
	}
	existing, hasExisting, err := loadProjectSourceForUpdate(tx, projectID)
	if err != nil {
		return false, err
	}
	config, err := buildRemoteSourceConfig(existing, hasExisting, input)
	if err != nil {
		return false, err
	}
	if !hasExisting {
		return true, createRemoteSourceTx(tx, projectID, config)
	}
	return updateExistingRemoteSourceTx(tx, existing, config)
}

func loadProjectSourceForUpdate(tx *gorm.DB, projectID uint) (*model.PagesProjectSource, bool, error) {
	source, err := repository.LockPagesProjectSourceByProjectIDTx(tx, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.PagesProjectSource{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return source, true, nil
}

func buildRemoteSourceConfig(
	_ *model.PagesProjectSource,
	_ bool,
	input SourceUpdateInput,
) (remoteSourceConfig, error) {
	remoteURL := strings.TrimSpace(input.RemoteURL)
	if remoteURL == "" {
		return remoteSourceConfig{}, errors.New(errPagesSourceRemoteURLRequired)
	}
	parsedURL, err := parseRemoteSourceURL(remoteURL)
	if err != nil {
		return remoteSourceConfig{}, err
	}
	return remoteSourceConfig{
		URL:           remoteURL,
		AllowInsecure: input.AllowInsecure,
		Identity:      remoteSourceIdentity(parsedURL),
	}, nil
}

func createRemoteSourceTx(tx *gorm.DB, projectID uint, config remoteSourceConfig) error {
	source := &model.PagesProjectSource{
		ProjectID:            projectID,
		SourceType:           PagesSourceTypeRemoteURL,
		RemoteURL:            config.URL,
		AllowInsecure:        config.AllowInsecure,
		AutoUpdateEnabled:    false,
		CheckIntervalMinutes: 0,
		ConfigVersion:        1,
		SourceIdentity:       config.Identity,
	}
	if err := repository.CreatePagesProjectSourceTx(tx, source); err != nil {
		return err
	}
	return repository.CreatePagesProjectSourceRuntimeTx(tx, &model.PagesProjectSourceRuntime{
		SourceID:   source.ID,
		SyncStatus: pagesSourceStatusIdle,
	})
}

func updateExistingRemoteSourceTx(
	tx *gorm.DB,
	existing *model.PagesProjectSource,
	config remoteSourceConfig,
) (bool, error) {
	if !remoteSourceConfigChanged(existing, config) {
		return false, nil
	}
	runtime, err := repository.LockPagesProjectSourceRuntimeBySourceIDTx(tx, existing.ID)
	if err != nil {
		return false, err
	}
	identityChanged := existing.SourceIdentity != config.Identity
	if err := repository.UpdatePagesProjectSourceTx(tx, existing, map[string]any{
		"source_type":                 PagesSourceTypeRemoteURL,
		"remote_url":                  config.URL,
		"allow_insecure":              config.AllowInsecure,
		"github_repository":           "",
		"release_selector":            "",
		"release_tag":                 "",
		"asset_name":                  "",
		sourceColumnAutoUpdateEnabled: false,
		"check_interval_minutes":      0,
		sourceColumnConfigVersion:     existing.ConfigVersion + 1,
		"source_identity":             config.Identity,
	}); err != nil {
		return false, err
	}
	return true, resetRuntimeAfterSourceUpdate(tx, runtime, identityChanged)
}

func remoteSourceConfigChanged(existing *model.PagesProjectSource, config remoteSourceConfig) bool {
	return existing.SourceType != PagesSourceTypeRemoteURL ||
		existing.RemoteURL != config.URL ||
		existing.AllowInsecure != config.AllowInsecure ||
		existing.GitHubRepository != "" ||
		existing.ReleaseSelector != "" ||
		existing.ReleaseTag != "" ||
		existing.AssetName != "" ||
		existing.AutoUpdateEnabled ||
		existing.CheckIntervalMinutes != 0
}

// DeleteSource idempotently switches a project back to manual mode.
func DeleteSource(ctx context.Context, projectID uint) (*SourceView, error) {
	err := repository.WithPagesTx(ctx, func(tx *gorm.DB) error {
		if _, err := repository.LockPagesProjectByIDTx(tx, projectID); err != nil {
			return err
		}
		source, err := repository.LockPagesProjectSourceByProjectIDTx(tx, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := repository.LockPagesProjectSourceRuntimeBySourceIDTx(tx, source.ID); err != nil &&
			!errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := repository.DeletePagesProjectSourceRuntimeBySourceIDTx(tx, source.ID); err != nil {
			return err
		}
		return repository.DeletePagesProjectSourceTx(tx, source)
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
	if strings.TrimSpace(input.RemoteURL) == "" {
		return errors.New(errPagesSourceRemoteURLRequired)
	}
	return nil
}

func validateSourceUpdateInput(input SourceUpdateInput) error {
	switch strings.TrimSpace(input.SourceType) {
	case PagesSourceTypeRemoteURL:
		return validateRemoteSourceInput(input)
	case PagesSourceTypeGitHubRelease:
		return validateGitHubSourceInput(input)
	case "":
		return errors.New(errPagesSourceTypeRequired)
	default:
		return errors.New(errPagesSourceTypeUnsupported)
	}
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

func loadSourceByProject(ctx context.Context, projectID uint) (*model.PagesProjectSource, *model.PagesProjectSourceRuntime, error) {
	return repository.GetPagesProjectSourceAndRuntimeByProjectID(ctx, projectID)
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
		view.RemoteURL = source.RemoteURL
		view.AllowInsecure = source.AllowInsecure
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
		autoUpdateEnabled := source.AutoUpdateEnabled
		view.AutoUpdateEnabled = &autoUpdateEnabled
		view.CheckIntervalMinutes = source.CheckIntervalMinutes
	default:
		return nil, errors.New(errPagesSourceTypeUnsupported)
	}
	return view, nil
}

func revisionView(revision string, detailJSON string) *SourceRevisionView {
	detail := sourceDetail{}
	_ = unmarshalSourceDetail(detailJSON, &detail)
	label := sourceDetailLabel(detail)
	if label == "" {
		label = defaultRemoteAssetLabel
	}
	return &SourceRevisionView{
		Revision:  revision,
		Label:     label,
		AssetName: detail.AssetName,
	}
}

func sourceDetailLabel(detail sourceDetail) string {
	if detail.Provider == githubSourceDetailProvider {
		if label := strings.TrimSpace(detail.Tag); label != "" {
			return label
		}
		return strings.TrimSpace(detail.LegacyLabel)
	}
	if label := strings.TrimSpace(detail.DisplayName); label != "" {
		return label
	}
	return strings.TrimSpace(detail.LegacyLabel)
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
	return repository.UpdatePagesProjectSourceRuntimeTx(tx, runtime, updates)
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
