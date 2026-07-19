// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	githubReleaseSelectorLatest = "latest"
	githubReleaseSelectorTag    = "tag"
	githubSourceIdentityDomain  = "openflare:pages:github-release:v2"
	initialCheckRetryDelay      = 5 * time.Minute
	githubRepositoryPathParts   = 2
	githubCheckJitterRange      = 301
	githubCheckJitterCenter     = 150
)

var (
	githubOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	githubRepoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type githubSourceConfig struct {
	Repository     string
	Selector       string
	Tag            string
	AssetName      string
	AutoUpdate     bool
	CheckInterval  int
	SourceIdentity string
}

func validateGitHubSourceInput(input SourceUpdateInput) error {
	if strings.TrimSpace(input.SourceType) != PagesSourceTypeGitHubRelease {
		return errors.New(errPagesSourceTypeUnsupported)
	}
	if input.RemoteURLSet || strings.TrimSpace(input.RemoteURL) != "" ||
		strings.TrimSpace(input.RemoteNetworkPolicy) != "" {
		return errors.New(errPagesSourceGitHubFields)
	}
	if _, err := normalizeGitHubRepositoryURL(input.RepositoryURL); err != nil {
		return err
	}
	selector := strings.TrimSpace(input.ReleaseSelector)
	if selector == "" {
		selector = githubReleaseSelectorLatest
	}
	assetName := input.AssetName
	if assetName == "" {
		assetName = defaultGitHubAssetName
	}
	if !validGitHubAssetName(assetName) {
		return errors.New(errPagesSourceAssetNameInvalid)
	}
	switch selector {
	case githubReleaseSelectorLatest:
		if input.ReleaseTag != "" {
			return errors.New(errPagesSourceSelectorInvalid)
		}
		interval := input.CheckIntervalMinutes
		if interval != 0 && (interval < minimumCheckInterval || interval > maximumCheckInterval) {
			return errors.New(errPagesSourceCheckInterval)
		}
	case githubReleaseSelectorTag:
		if !validGitHubReleaseTagConfig(input.ReleaseTag) || input.AutoUpdateEnabled || input.CheckIntervalMinutes != 0 {
			return errors.New(errPagesSourceSelectorInvalid)
		}
	default:
		return errors.New(errPagesSourceSelectorInvalid)
	}
	return nil
}

func buildGitHubSourceConfig(input SourceUpdateInput) (githubSourceConfig, error) {
	repository, err := normalizeGitHubRepositoryURL(input.RepositoryURL)
	if err != nil {
		return githubSourceConfig{}, err
	}
	selector := strings.TrimSpace(input.ReleaseSelector)
	if selector == "" {
		selector = githubReleaseSelectorLatest
	}
	tag := input.ReleaseTag
	assetName := input.AssetName
	if assetName == "" {
		assetName = defaultGitHubAssetName
	}
	interval := input.CheckIntervalMinutes
	if selector == githubReleaseSelectorLatest && interval == 0 {
		interval = defaultCheckInterval
	}
	autoUpdate := input.AutoUpdateEnabled
	if selector == githubReleaseSelectorTag {
		autoUpdate = false
		interval = 0
	}
	return githubSourceConfig{
		Repository:     repository,
		Selector:       selector,
		Tag:            tag,
		AssetName:      assetName,
		AutoUpdate:     autoUpdate,
		CheckInterval:  interval,
		SourceIdentity: buildGitHubSourceIdentity(repository, selector, tag, assetName),
	}, nil
}

func buildGitHubSourceIdentity(repository, selector, tag, assetName string) string {
	fields := [...]string{repository, selector, tag, assetName}
	encoded := make([]byte, 0, len(githubSourceIdentityDomain)+len(fields)*8+
		len(repository)+len(selector)+len(tag)+len(assetName))
	encoded = append(encoded, githubSourceIdentityDomain...)
	var fieldLength [8]byte
	for _, field := range fields {
		// Go strings hold the validated UTF-8 bytes used by GitHub. Prefixing each
		// field with its byte length prevents delimiter characters from creating
		// ambiguous identities across field boundaries.
		binary.BigEndian.PutUint64(fieldLength[:], uint64(len(field)))
		encoded = append(encoded, fieldLength[:]...)
		encoded = append(encoded, field...)
	}
	identityHash := sha256.Sum256(encoded)
	return hex.EncodeToString(identityHash[:])
}

func normalizeGitHubRepositoryURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		strings.Contains(raw, "#") ||
		parsed.EscapedPath() != parsed.Path || !strings.HasPrefix(parsed.Path, "/") ||
		strings.HasPrefix(parsed.Path, "//") || strings.HasSuffix(parsed.Path, "/") {
		return "", errors.New(errPagesSourceRepositoryInvalid)
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(parts) != githubRepositoryPathParts {
		return "", errors.New(errPagesSourceRepositoryInvalid)
	}
	owner := parts[0]
	repository := parts[1]
	repository = strings.TrimSuffix(repository, ".git")
	if !githubOwnerPattern.MatchString(owner) || !githubRepoPattern.MatchString(repository) ||
		len(repository) > 100 || repository == "." || repository == ".." {
		return "", errors.New(errPagesSourceRepositoryInvalid)
	}
	return owner + "/" + repository, nil
}

func validGitHubReleaseTagConfig(value string) bool {
	if !validGitHubReleaseDisplayTag(value) ||
		strings.ContainsAny(value, " ~^:?*[\\") || strings.HasPrefix(value, "/") ||
		strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".") ||
		strings.Contains(value, "//") || strings.Contains(value, "..") || strings.Contains(value, "@{") {
		return false
	}
	for component := range strings.SplitSeq(value, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validGitHubReleaseDisplayTag(value string) bool {
	if value == "" || len(value) > 255 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unsafeGitHubInputRune(character) {
			return false
		}
	}
	return true
}

func validGitHubAssetName(value string) bool {
	if value == "" || len(value) > 255 || !utf8.ValidString(value) ||
		path.Base(value) != value || strings.Contains(value, "\\") ||
		value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if unsafeGitHubInputRune(character) {
			return false
		}
	}
	return true
}

func unsafeGitHubInputRune(character rune) bool {
	return unicode.IsControl(character) || character == '\u2028' || character == '\u2029' ||
		character == '\u061c' || character == '\u200e' || character == '\u200f' ||
		(character >= '\u202a' && character <= '\u202e') ||
		(character >= '\u2066' && character <= '\u2069')
}

func updateGitHubSourceTx(tx *gorm.DB, projectID uint, input SourceUpdateInput) (bool, error) {
	var project model.PagesProject
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, projectID).Error; err != nil {
		return false, err
	}
	existing, hasExisting, err := loadProjectSourceForUpdate(tx, projectID)
	if err != nil {
		return false, err
	}
	config, err := buildGitHubSourceConfig(input)
	if err != nil {
		return false, err
	}
	if !hasExisting {
		return true, createGitHubSourceTx(tx, projectID, config)
	}
	if !githubSourceConfigChanged(existing, config) {
		return false, nil
	}
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", existing.ID).First(&runtime).Error; err != nil {
		return false, err
	}
	identityChanged := existing.SourceIdentity != config.SourceIdentity
	if err := tx.Model(existing).Updates(githubSourceUpdates(config, existing.ConfigVersion+1)).Error; err != nil {
		return false, err
	}
	if err := resetRuntimeAfterGitHubUpdate(tx, &runtime, config, identityChanged); err != nil {
		return false, err
	}
	return true, nil
}

func createGitHubSourceTx(tx *gorm.DB, projectID uint, config githubSourceConfig) error {
	source := &model.PagesProjectSource{
		ProjectID:            projectID,
		SourceType:           PagesSourceTypeGitHubRelease,
		GitHubRepository:     config.Repository,
		ReleaseSelector:      config.Selector,
		ReleaseTag:           config.Tag,
		AssetName:            config.AssetName,
		AutoUpdateEnabled:    config.AutoUpdate,
		CheckIntervalMinutes: config.CheckInterval,
		ConfigVersion:        1,
		SourceIdentity:       config.SourceIdentity,
	}
	if err := tx.Create(source).Error; err != nil {
		return err
	}
	runtime := &model.PagesProjectSourceRuntime{SourceID: source.ID, SyncStatus: pagesSourceStatusIdle}
	if config.Selector == githubReleaseSelectorLatest {
		next := nextGitHubCheckAt(time.Now(), source.ID, config.CheckInterval)
		runtime.NextCheckAt = &next
	}
	return tx.Create(runtime).Error
}

func githubSourceUpdates(config githubSourceConfig, version int) map[string]any {
	return map[string]any{
		"source_type":                 PagesSourceTypeGitHubRelease,
		"remote_url":                  "",
		"remote_network_policy":       "",
		"github_repository":           config.Repository,
		"release_selector":            config.Selector,
		"release_tag":                 config.Tag,
		"asset_name":                  config.AssetName,
		sourceColumnAutoUpdateEnabled: config.AutoUpdate,
		"check_interval_minutes":      config.CheckInterval,
		sourceColumnConfigVersion:     version,
		"source_identity":             config.SourceIdentity,
	}
}

func githubSourceConfigChanged(existing *model.PagesProjectSource, config githubSourceConfig) bool {
	return existing.SourceType != PagesSourceTypeGitHubRelease || existing.RemoteURL != "" ||
		existing.RemoteNetworkPolicy != "" || existing.GitHubRepository != config.Repository ||
		existing.ReleaseSelector != config.Selector || existing.ReleaseTag != config.Tag ||
		existing.AssetName != config.AssetName || existing.AutoUpdateEnabled != config.AutoUpdate ||
		existing.CheckIntervalMinutes != config.CheckInterval
}

func resetRuntimeAfterGitHubUpdate(
	tx *gorm.DB,
	runtime *model.PagesProjectSourceRuntime,
	config githubSourceConfig,
	identityChanged bool,
) error {
	if err := resetRuntimeAfterSourceUpdate(tx, runtime, identityChanged); err != nil {
		return err
	}
	var nextCheckAt any
	if config.Selector == githubReleaseSelectorLatest {
		next := nextGitHubCheckAt(time.Now(), runtime.SourceID, config.CheckInterval)
		nextCheckAt = &next
	}
	return tx.Model(runtime).Update("next_check_at", nextCheckAt).Error
}

func nextGitHubCheckAt(now time.Time, sourceID uint, intervalMinutes int) time.Time {
	// A stable, bounded offset avoids a thundering herd without persisting
	// another scheduling field. Scanner Phase 3 reuses this calculation.
	jitterSeconds := int64(sourceID%githubCheckJitterRange) - githubCheckJitterCenter
	return now.Add(time.Duration(intervalMinutes)*time.Minute + time.Duration(jitterSeconds)*time.Second)
}

func markInitialCheckDispatchFailed(ctx context.Context, sourceID uint, configVersion int) {
	updates := map[string]any{
		sourceRuntimeColumnSyncStatus: pagesSourceStatusFailed,
		sourceRuntimeColumnLastError:  errPagesSourceInitialCheckWarning,
	}
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ? AND config_version = ?", sourceID, configVersion).First(&source).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.ErrorF(ctx, "[PagesSource] load initial check source snapshot failed: source_id=%d error=%v", sourceID, err)
		}
		return
	}
	if source.ReleaseSelector == githubReleaseSelectorLatest {
		next := time.Now().Add(initialCheckRetryDelay)
		updates["next_check_at"] = &next
	}
	now := time.Now()
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", sourceID).
		Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Where("EXISTS (SELECT 1 FROM of_pages_project_sources source WHERE source.id = ? AND source.config_version = ?)", sourceID, configVersion).
		Updates(updates)
	if result.Error != nil {
		logger.ErrorF(ctx, "[PagesSource] mark initial check dispatch failure: source_id=%d error=%v", sourceID, result.Error)
	}
}
