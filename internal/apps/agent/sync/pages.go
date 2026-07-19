// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package sync applies control-plane configuration to the local agent runtime.
package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
)

const (
	pagesDirPerm              = 0o755
	pagesFilePerm             = 0o644
	pagesManifestFilePerm     = 0o644
	agentPagesMaxPackageBytes = int64(2 * 1024 * 1024 * 1024)
	agentPagesMaxFiles        = 1000
	agentPagesMaxFileBytes    = int64(8 * 1024 * 1024 * 1024)
	agentPagesMaxTotalBytes   = int64(8 * 1024 * 1024 * 1024)
	// pagesLatestPullAttempts covers a race where the active deployment changes
	// between the hash probe and the package download.
	pagesLatestPullAttempts = 2
)

type pagesSourceDocument struct {
	Routes []pagesSourceRoute `json:"routes"`
}

type pagesSourceRoute struct {
	UpstreamType    string                 `json:"upstream_type"`
	PagesProjectID  *uint                  `json:"pages_project_id"`
	PagesDeployment *pagesDeploymentSource `json:"pages_deployment"`
}

// pagesProjectRef is the agent-side "latest" pointer for one Pages project.
type pagesProjectRef struct {
	ProjectID    uint
	DeploymentID uint
	Checksum     string
}

type pagesPackageLimits struct {
	PackageBytes int64
	Extraction   pagesarchive.Limits
}

type pagesDeploymentMarker struct {
	ProjectID    uint   `json:"project_id"`
	DeploymentID uint   `json:"deployment_id,omitempty"`
	Checksum     string `json:"checksum"`
}

func pagesDeploymentStateHash(item state.PagesDeployment) string {
	if hash := strings.TrimSpace(item.Hash); hash != "" {
		return hash
	}
	return strings.TrimSpace(item.Checksum)
}

func snapshotPagesProjects(snapshot *state.Snapshot) []pagesProjectRef {
	if snapshot == nil || snapshot.PagesDeployments == nil {
		return nil
	}
	result := make([]pagesProjectRef, 0, len(snapshot.PagesDeployments))
	for _, item := range snapshot.PagesDeployments {
		projectID := item.ProjectID
		if projectID == 0 {
			// Legacy agent state only stored deployment_id; skip until rediscovered from config.
			continue
		}
		result = append(result, pagesProjectRef{
			ProjectID:    projectID,
			DeploymentID: item.DeploymentID,
			Checksum:     pagesDeploymentStateHash(item),
		})
	}
	return result
}

func setSnapshotPagesProjects(snapshot *state.Snapshot, projects []pagesProjectRef) {
	if snapshot == nil {
		return
	}
	if len(projects) == 0 {
		snapshot.PagesDeployments = []state.PagesDeployment{}
		return
	}
	snapshot.PagesDeployments = make([]state.PagesDeployment, len(projects))
	for i, project := range projects {
		snapshot.PagesDeployments[i] = state.PagesDeployment{
			ProjectID:    project.ProjectID,
			DeploymentID: project.DeploymentID,
			Hash:         strings.TrimSpace(project.Checksum),
		}
	}
}

func updateSnapshotPagesProject(snapshot *state.Snapshot, project pagesProjectRef) {
	if snapshot == nil || snapshot.PagesDeployments == nil {
		return
	}
	hash := strings.TrimSpace(project.Checksum)
	for i := range snapshot.PagesDeployments {
		if snapshot.PagesDeployments[i].ProjectID != project.ProjectID {
			continue
		}
		snapshot.PagesDeployments[i].DeploymentID = project.DeploymentID
		snapshot.PagesDeployments[i].Hash = hash
		snapshot.PagesDeployments[i].Checksum = ""
		return
	}
}

func pagesDiscoveryNeeded(snapshot *state.Snapshot) bool {
	if snapshot == nil || snapshot.PagesDeployments == nil {
		return true
	}
	// Legacy state rows may only have deployment_id (project_id == 0). Those
	// cannot poll latest-by-project; force a full config rediscovery.
	if len(snapshot.PagesDeployments) > 0 && len(snapshotPagesProjects(snapshot)) == 0 {
		return true
	}
	return false
}

func pagesSyncNeeded(snapshot *state.Snapshot) bool {
	return snapshot != nil && len(snapshotPagesProjects(snapshot)) > 0
}

func pagesReconcileNeeded(snapshot *state.Snapshot) bool {
	if pagesDiscoveryNeeded(snapshot) {
		return true
	}
	return pagesSyncNeeded(snapshot)
}

func (s *Service) syncPagesDeployments(ctx context.Context, snapshot *state.Snapshot, config *protocol.ActiveConfigResponse) error {
	var projects []pagesProjectRef
	var err error
	if config != nil {
		projects, err = referencedPagesProjects(config)
		if err != nil {
			return err
		}
		setSnapshotPagesProjects(snapshot, projects)
	} else {
		projects = snapshotPagesProjects(snapshot)
	}
	if len(projects) == 0 {
		return nil
	}
	if strings.TrimSpace(s.pagesDir) == "" {
		return errors.New("pages_dir is required when active config references Pages projects")
	}

	// Isolate per-project failures so one bad project does not block others.
	var failed []error
	for _, project := range projects {
		if ensureErr := s.ensurePagesProject(ctx, snapshot, project.ProjectID); ensureErr != nil {
			slog.Error("ensure Pages project failed",
				"project_id", project.ProjectID,
				"error", ensureErr,
			)
			failed = append(failed, fmt.Errorf("pages project %d: %w", project.ProjectID, ensureErr))
		}
	}
	if s.nginxManager != nil {
		if accessErr := s.nginxManager.EnsureWorkerReadAccess(); accessErr != nil {
			failed = append(failed, fmt.Errorf("ensure openresty worker read access: %w", accessErr))
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return errors.Join(failed...)
}

// ensurePagesProject pulls the control-plane "latest" (active) package for a
// Pages project and switches local current to that release when needed.
// Only the latest release is retained on disk; older releases are removed after
// the new release is ready and current has been switched.
func (s *Service) ensurePagesProject(ctx context.Context, snapshot *state.Snapshot, projectID uint) error {
	if projectID == 0 {
		return errors.New("pages project id is required")
	}

	var lastErr error
	for attempt := 0; attempt < pagesLatestPullAttempts; attempt++ {
		latest, err := s.client.GetPagesProjectLatestHash(ctx, projectID)
		if err != nil {
			return fmt.Errorf("fetch Pages project %d latest hash: %w", projectID, err)
		}
		limits, err := validatePagesPackageMetadata(projectID, latest)
		if err != nil {
			return err
		}
		hash := strings.TrimSpace(latest.Hash)
		effective := pagesProjectRef{
			ProjectID:    projectID,
			DeploymentID: latest.DeploymentID,
			Checksum:     hash,
		}

		releaseDir := pagesProjectReleaseDir(s.pagesDir, projectID, hash)
		if pagesProjectReleaseReady(releaseDir, effective) {
			if err := switchPagesProjectCurrentDir(s.pagesDir, projectID, releaseDir); err != nil {
				return err
			}
			updateSnapshotPagesProject(snapshot, effective)
			_ = cleanupPagesProjectStaleReleases(s.pagesDir, projectID, hash)
			return nil
		}

		packagePath, got, err := s.downloadPagesProjectPackage(ctx, projectID, latest, limits.PackageBytes)
		if err != nil {
			return fmt.Errorf("download Pages project %d latest package: %w", projectID, err)
		}

		// Re-probe latest after download to detect activation races.
		// A deployment-id-only change is still a latest-pointer race even when
		// deduplication makes both deployments share the same package hash.
		verify, err := s.client.GetPagesProjectLatestHash(ctx, projectID)
		if err != nil {
			_ = os.Remove(packagePath)
			return fmt.Errorf("re-fetch Pages project %d latest hash: %w", projectID, err)
		}
		if _, err := validatePagesPackageMetadata(projectID, verify); err != nil {
			_ = os.Remove(packagePath)
			return err
		}
		if !samePagesPackageMetadata(latest, verify) {
			_ = os.Remove(packagePath)
			lastErr = fmt.Errorf(
				"pages project %d latest metadata changed during download: deployment %d/%s -> %d/%s (attempt %d/%d)",
				projectID,
				latest.DeploymentID,
				strings.TrimSpace(latest.Hash),
				verify.DeploymentID,
				strings.TrimSpace(verify.Hash),
				attempt+1,
				pagesLatestPullAttempts,
			)
			slog.Warn("pages latest metadata race, retrying",
				"project_id", projectID,
				"before_deployment_id", latest.DeploymentID,
				"before_hash", strings.TrimSpace(latest.Hash),
				"after_deployment_id", verify.DeploymentID,
				"after_hash", strings.TrimSpace(verify.Hash),
				"attempt", attempt+1,
			)
			continue
		}
		if got != hash {
			_ = os.Remove(packagePath)
			lastErr = fmt.Errorf(
				"pages project %d package hash mismatch: downloaded %s, expected %s (attempt %d/%d)",
				projectID, got, hash, attempt+1, pagesLatestPullAttempts,
			)
			slog.Warn("pages latest package hash mismatch, retrying",
				"project_id", projectID,
				"downloaded_hash", got,
				"expected_hash", hash,
				"attempt", attempt+1,
			)
			continue
		}

		releaseDir = pagesProjectReleaseDir(s.pagesDir, projectID, got)
		extractErr := extractPagesPackageFile(packagePath, releaseDir, effective, limits.Extraction, latest)
		_ = os.Remove(packagePath)
		if extractErr != nil {
			return extractErr
		}
		if err := switchPagesProjectCurrentDir(s.pagesDir, projectID, releaseDir); err != nil {
			return err
		}
		updateSnapshotPagesProject(snapshot, effective)
		// Only after the new release is ready and current switched: drop others.
		_ = cleanupPagesProjectStaleReleases(s.pagesDir, projectID, got)
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("pages project %d latest pull failed", projectID)
}

func validatePagesPackageMetadata(
	projectID uint,
	metadata *protocol.PagesProjectLatestHashResponse,
) (pagesPackageLimits, error) {
	if metadata == nil {
		return pagesPackageLimits{}, fmt.Errorf("pages project %d latest metadata is missing", projectID)
	}
	if metadata.ProjectID != projectID {
		return pagesPackageLimits{}, fmt.Errorf(
			"pages project %d latest metadata has project id %d",
			projectID,
			metadata.ProjectID,
		)
	}
	if metadata.DeploymentID == 0 {
		return pagesPackageLimits{}, fmt.Errorf("pages project %d latest deployment id is missing", projectID)
	}
	if strings.TrimSpace(metadata.Hash) == "" {
		return pagesPackageLimits{}, fmt.Errorf("pages project %d latest hash is empty", projectID)
	}
	if metadata.PackageSize <= 0 {
		return pagesPackageLimits{}, fmt.Errorf("pages project %d package size must be positive", projectID)
	}
	if metadata.PackageSize > agentPagesMaxPackageBytes {
		return pagesPackageLimits{}, fmt.Errorf(
			"pages project %d package size %d exceeds agent limit %d",
			projectID,
			metadata.PackageSize,
			agentPagesMaxPackageBytes,
		)
	}
	if metadata.FileCount <= 0 {
		return pagesPackageLimits{}, fmt.Errorf("pages project %d file count must be positive", projectID)
	}
	if metadata.FileCount > agentPagesMaxFiles {
		return pagesPackageLimits{}, fmt.Errorf(
			"pages project %d file count %d exceeds agent limit %d",
			projectID,
			metadata.FileCount,
			agentPagesMaxFiles,
		)
	}
	if metadata.TotalSize < 0 {
		return pagesPackageLimits{}, fmt.Errorf("pages project %d total size cannot be negative", projectID)
	}
	if metadata.TotalSize > agentPagesMaxTotalBytes {
		return pagesPackageLimits{}, fmt.Errorf(
			"pages project %d total size %d exceeds agent limit %d",
			projectID,
			metadata.TotalSize,
			agentPagesMaxTotalBytes,
		)
	}

	// pagesarchive treats zero limits as defaults. A one-byte extraction guard
	// plus the exact post-extraction manifest check below preserves the valid
	// case of one or more zero-byte files while still enforcing total_size=0.
	extractedBytes := metadata.TotalSize
	if extractedBytes == 0 {
		extractedBytes = 1
	}
	maxFileBytes := extractedBytes
	if maxFileBytes > agentPagesMaxFileBytes {
		maxFileBytes = agentPagesMaxFileBytes
	}

	return pagesPackageLimits{
		PackageBytes: metadata.PackageSize,
		Extraction: pagesarchive.Limits{
			MaxFiles:      metadata.FileCount,
			MaxFileBytes:  maxFileBytes,
			MaxTotalBytes: extractedBytes,
		},
	}, nil
}

func samePagesPackageMetadata(
	before *protocol.PagesProjectLatestHashResponse,
	after *protocol.PagesProjectLatestHashResponse,
) bool {
	if before == nil || after == nil {
		return false
	}
	return before.ProjectID == after.ProjectID &&
		before.DeploymentID == after.DeploymentID &&
		strings.TrimSpace(before.Hash) == strings.TrimSpace(after.Hash) &&
		before.PackageSize == after.PackageSize &&
		before.FileCount == after.FileCount &&
		before.TotalSize == after.TotalSize
}

func (s *Service) downloadPagesProjectPackage(
	ctx context.Context,
	projectID uint,
	metadata *protocol.PagesProjectLatestHashResponse,
	maxBytes int64,
) (packagePath string, hash string, err error) {
	releasesRoot := filepath.Join(s.pagesDir, "projects", fmt.Sprintf("%d", projectID), "releases")
	if err := os.MkdirAll(releasesRoot, pagesDirPerm); err != nil {
		return "", "", err
	}
	packageFile, err := os.CreateTemp(releasesRoot, ".package-*.tmp")
	if err != nil {
		return "", "", err
	}
	packagePath = packageFile.Name()
	keep := false
	defer func() {
		if closeErr := packageFile.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if !keep || err != nil {
			_ = os.Remove(packagePath)
			packagePath = ""
		}
	}()

	hasher := sha256.New()
	written, err := s.client.DownloadPagesProjectLatestPackage(
		ctx,
		projectID,
		io.MultiWriter(packageFile, hasher),
		maxBytes,
	)
	if err != nil {
		return "", "", err
	}
	if written != metadata.PackageSize {
		return "", "", fmt.Errorf(
			"pages project %d package size %d does not match metadata %d",
			projectID,
			written,
			metadata.PackageSize,
		)
	}
	keep = true
	return packagePath, hex.EncodeToString(hasher.Sum(nil)), nil
}

// cleanupPagesProjectStaleReleases keeps only keepHash under projects/{id}/releases.
// Must be called only after the keepHash release is ready and current points at it.
func cleanupPagesProjectStaleReleases(baseDir string, projectID uint, keepHash string) error {
	keepHash = strings.TrimSpace(keepHash)
	if projectID == 0 || keepHash == "" {
		return nil
	}
	releasesRoot := filepath.Join(baseDir, "projects", fmt.Sprintf("%d", projectID), "releases")
	entries, err := os.ReadDir(releasesRoot) //nolint:gosec // managed PagesDir
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var firstErr error
	for _, entry := range entries {
		name := entry.Name()
		if name == keepHash {
			continue
		}
		// Drop partial extract leftovers as well (*.tmp).
		target := filepath.Join(releasesRoot, name)
		if removeErr := os.RemoveAll(target); removeErr != nil && firstErr == nil {
			firstErr = removeErr
			slog.Warn("failed to remove stale Pages release",
				"project_id", projectID,
				"path", target,
				"error", removeErr,
			)
		}
	}
	// Also remove legacy deployments/ tree leftovers if present (best-effort).
	_ = os.RemoveAll(filepath.Join(baseDir, "deployments"))
	return firstErr
}

func pagesProjectReleaseReady(dir string, project pagesProjectRef) bool {
	if !markerMatches(dir, project) {
		return false
	}
	entries, err := os.ReadDir(dir) //nolint:gosec // dir is managed PagesDir
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == ".openflare-pages.json" {
			continue
		}
		return true
	}
	return false
}

func referencedPagesProjects(config *protocol.ActiveConfigResponse) ([]pagesProjectRef, error) {
	if config == nil || strings.TrimSpace(config.SourceConfigJSON) == "" {
		return nil, nil
	}
	var doc pagesSourceDocument
	if err := json.Unmarshal([]byte(config.SourceConfigJSON), &doc); err != nil {
		return nil, fmt.Errorf("decode pages references: %w", err)
	}
	seen := make(map[uint]struct{})
	result := make([]pagesProjectRef, 0)
	for _, route := range doc.Routes {
		if strings.ToLower(strings.TrimSpace(route.UpstreamType)) != "pages" {
			continue
		}
		projectID := pagesProjectIDFromRoute(route)
		if projectID == 0 {
			return nil, errors.New("pages route is missing project_id")
		}
		if _, ok := seen[projectID]; ok {
			continue
		}
		seen[projectID] = struct{}{}
		checksum := ""
		deploymentID := uint(0)
		if route.PagesDeployment != nil {
			checksum = strings.TrimSpace(route.PagesDeployment.Checksum)
			deploymentID = route.PagesDeployment.DeploymentID
		}
		result = append(result, pagesProjectRef{
			ProjectID:    projectID,
			DeploymentID: deploymentID,
			Checksum:     checksum,
		})
	}
	return result, nil
}

func pagesProjectIDFromRoute(route pagesSourceRoute) uint {
	if route.PagesProjectID != nil && *route.PagesProjectID != 0 {
		return *route.PagesProjectID
	}
	if route.PagesDeployment != nil && route.PagesDeployment.ProjectID != 0 {
		return route.PagesDeployment.ProjectID
	}
	return 0
}

// pagesDeploymentSource is the subset of pages_deployment used when parsing config.
type pagesDeploymentSource struct {
	ProjectID    uint   `json:"project_id"`
	DeploymentID uint   `json:"deployment_id"`
	Checksum     string `json:"checksum"`
}

func extractPagesPackageFile(
	packagePath string,
	releaseDir string,
	project pagesProjectRef,
	limits pagesarchive.Limits,
	expected *protocol.PagesProjectLatestHashResponse,
) error {
	if err := os.MkdirAll(filepath.Dir(releaseDir), pagesDirPerm); err != nil {
		return err
	}
	stagingDir, err := os.MkdirTemp(
		filepath.Dir(releaseDir),
		"."+filepath.Base(releaseDir)+"-*.tmp",
	)
	if err != nil {
		return err
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			removePagesStagingUnlessCurrent(stagingDir, pagesCurrentDirFromRelease(releaseDir))
		}
	}()

	if err := pagesarchive.ExtractFile(packagePath, "", stagingDir, pagesarchive.ExtractOptions{
		StripCommonRoot: true,
		EnforceLimits:   true,
		Limits:          limits,
	}); err != nil {
		return fmt.Errorf("extract Pages package: %w", err)
	}
	if err := validateExtractedPagesMetadata(stagingDir, expected); err != nil {
		return err
	}
	if err := writePagesMarker(stagingDir, project); err != nil {
		return err
	}
	if err := promotePagesRelease(stagingDir, releaseDir, project); err != nil {
		return err
	}
	cleanupStaging = false
	return nil
}

func validateExtractedPagesMetadata(
	dir string,
	expected *protocol.PagesProjectLatestHashResponse,
) error {
	if expected == nil {
		return nil
	}
	fileCount := 0
	totalSize := int64(0)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pages extracted entry is not a regular file: %s", path)
		}
		fileCount++
		if fileCount > agentPagesMaxFiles {
			return fmt.Errorf("pages extracted file count exceeds agent limit %d", agentPagesMaxFiles)
		}
		if info.Size() < 0 || info.Size() > agentPagesMaxTotalBytes-totalSize {
			return fmt.Errorf("pages extracted size exceeds agent limit %d", agentPagesMaxTotalBytes)
		}
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate extracted Pages package: %w", err)
	}
	if fileCount != expected.FileCount || totalSize != expected.TotalSize {
		return fmt.Errorf(
			"pages extracted metadata mismatch: got %d files/%d bytes, expected %d files/%d bytes",
			fileCount,
			totalSize,
			expected.FileCount,
			expected.TotalSize,
		)
	}
	return nil
}

func promotePagesRelease(stagingDir string, releaseDir string, project pagesProjectRef) error {
	return promotePagesReleaseWithCopy(stagingDir, releaseDir, project, copyPagesDir)
}

func promotePagesReleaseWithCopy(
	stagingDir string,
	releaseDir string,
	project pagesProjectRef,
	copyDir func(string, string) error,
) error {
	currentDir := pagesCurrentDirFromRelease(releaseDir)
	defer removePagesStagingUnlessCurrent(stagingDir, currentDir)
	currentUsesRelease, err := pagesCurrentTargetsRelease(currentDir, releaseDir)
	if err != nil {
		return err
	}
	if !currentUsesRelease {
		if err := os.RemoveAll(releaseDir); err != nil {
			return err
		}
		return os.Rename(stagingDir, releaseDir)
	}

	// A same-hash repair cannot remove releaseDir while current still resolves
	// through it. Keep traffic on the fully validated staging tree, rebuild the
	// canonical release, then atomically point current back to the canonical path.
	if err := switchPagesCurrentDir(currentDir, stagingDir, os.Rename); err != nil {
		return fmt.Errorf("switch Pages current to repair staging: %w", err)
	}
	backupDir := stagingDir + ".previous"
	if err := os.Rename(releaseDir, backupDir); err != nil {
		restoreErr := switchPagesCurrentDir(currentDir, releaseDir, os.Rename)
		return errors.Join(
			fmt.Errorf("move previous Pages release aside: %w", err),
			restoreErr,
		)
	}

	rollback := func(cause error) error {
		var rollbackErrors []error
		rollbackErrors = append(rollbackErrors, cause)
		if err := os.RemoveAll(releaseDir); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove failed Pages release repair: %w", err))
		}
		if err := os.Rename(backupDir, releaseDir); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore previous Pages release: %w", err))
			return errors.Join(rollbackErrors...)
		}
		if err := switchPagesCurrentDir(currentDir, releaseDir, os.Rename); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore previous Pages current target: %w", err))
		}
		return errors.Join(rollbackErrors...)
	}

	if err := copyDir(stagingDir, releaseDir); err != nil {
		return rollback(fmt.Errorf("copy repaired Pages release: %w", err))
	}
	if !pagesProjectReleaseReady(releaseDir, project) {
		return rollback(errors.New("repaired Pages release is not ready"))
	}
	if err := switchPagesCurrentDir(currentDir, releaseDir, os.Rename); err != nil {
		return rollback(fmt.Errorf("switch Pages current to repaired release: %w", err))
	}
	if err := os.RemoveAll(backupDir); err != nil {
		slog.Warn("failed to remove previous Pages release", "path", backupDir, "error", err)
	}
	if err := os.RemoveAll(stagingDir); err != nil {
		slog.Warn("failed to remove Pages repair staging", "path", stagingDir, "error", err)
	}
	return nil
}

func pagesCurrentDirFromRelease(releaseDir string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(releaseDir)), "current")
}

func pagesCurrentTargetsRelease(currentDir string, releaseDir string) (bool, error) {
	if _, err := os.Lstat(currentDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	currentInfo, err := os.Stat(currentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat Pages current target: %w", err)
	}
	releaseInfo, err := os.Stat(releaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return os.SameFile(currentInfo, releaseInfo), nil
}

func removePagesStagingUnlessCurrent(stagingDir string, currentDir string) {
	currentUsesStaging, err := pagesCurrentTargetsRelease(currentDir, stagingDir)
	if err == nil && currentUsesStaging {
		slog.Error("preserving Pages staging because current still references it", "path", stagingDir)
		return
	}
	if removeErr := os.RemoveAll(stagingDir); removeErr != nil {
		slog.Warn("failed to remove Pages staging", "path", stagingDir, "error", removeErr)
	}
}

func verifyPagesCurrentTarget(currentDir string, releaseDir string) error {
	currentInfo, err := os.Stat(currentDir)
	if err != nil {
		return fmt.Errorf("stat Pages current target: %w", err)
	}
	releaseInfo, err := os.Stat(releaseDir)
	if err != nil {
		return fmt.Errorf("stat Pages release target: %w", err)
	}
	if !os.SameFile(currentInfo, releaseInfo) {
		return fmt.Errorf("pages current target does not resolve to release %s", releaseDir)
	}
	return nil
}

func switchPagesCurrentDir(
	currentDir string,
	releaseDir string,
	rename func(string, string) error,
) error {
	return switchPagesCurrentDirWithOps(currentDir, releaseDir, rename, os.Symlink)
}

func switchPagesCurrentDirWithOps(
	currentDir string,
	releaseDir string,
	rename func(string, string) error,
	symlink func(string, string) error,
) error {
	if err := os.MkdirAll(filepath.Dir(currentDir), pagesDirPerm); err != nil {
		return err
	}
	currentInfo, currentErr := os.Lstat(currentDir)
	if currentErr != nil && !os.IsNotExist(currentErr) {
		return currentErr
	}
	if currentErr == nil && currentInfo.Mode()&os.ModeSymlink == 0 {
		return fallbackCopyPagesCurrentDir(currentDir, releaseDir, rename)
	}

	previousTarget := ""
	hadPrevious := currentErr == nil
	if hadPrevious {
		var err error
		previousTarget, err = os.Readlink(currentDir)
		if err != nil {
			return err
		}
	}

	relTarget, err := filepath.Rel(filepath.Dir(currentDir), releaseDir)
	if err != nil {
		relTarget = releaseDir
	}

	tmpSymlink := currentDir + ".tmp"
	if err := os.Remove(tmpSymlink); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := symlink(relTarget, tmpSymlink); err != nil {
		_ = os.Remove(tmpSymlink)
		return fallbackCopyPagesCurrentDir(currentDir, releaseDir, rename)
	}
	defer func() { _ = os.Remove(tmpSymlink) }()
	if err := verifyPagesCurrentTarget(tmpSymlink, releaseDir); err != nil {
		return err
	}
	if err := rename(tmpSymlink, currentDir); err != nil {
		return err
	}
	if err := verifyPagesCurrentTarget(currentDir, releaseDir); err != nil {
		rollbackErr := rollbackPagesCurrentSymlink(
			currentDir,
			previousTarget,
			hadPrevious,
			rename,
		)
		return errors.Join(err, rollbackErr)
	}
	return nil
}

func fallbackCopyPagesCurrentDir(
	currentDir string,
	releaseDir string,
	rename func(string, string) error,
) error {
	stagingDir := currentDir + ".copy.tmp"
	previousDir := currentDir + ".previous"
	if err := os.RemoveAll(stagingDir); err != nil {
		return err
	}
	if err := copyPagesDir(releaseDir, stagingDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return err
	}
	if err := os.RemoveAll(previousDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return err
	}

	hadPrevious := false
	if _, err := os.Lstat(currentDir); err == nil {
		if err := rename(currentDir, previousDir); err != nil {
			_ = os.RemoveAll(stagingDir)
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		_ = os.RemoveAll(stagingDir)
		return err
	}
	if err := rename(stagingDir, currentDir); err != nil {
		var restoreErr error
		if hadPrevious {
			restoreErr = rename(previousDir, currentDir)
		}
		_ = os.RemoveAll(stagingDir)
		return errors.Join(err, restoreErr)
	}
	if err := os.RemoveAll(previousDir); err != nil {
		slog.Warn("failed to remove previous Pages current directory", "path", previousDir, "error", err)
	}
	return nil
}

func switchPagesProjectCurrentDir(baseDir string, projectID uint, releaseDir string) error {
	return switchPagesProjectCurrentDirWithRename(baseDir, projectID, releaseDir, os.Rename)
}

func switchPagesProjectCurrentDirWithRename(
	baseDir string,
	projectID uint,
	releaseDir string,
	rename func(string, string) error,
) error {
	return switchPagesCurrentDir(pagesProjectCurrentDir(baseDir, projectID), releaseDir, rename)
}

func rollbackPagesCurrentSymlink(
	currentDir string,
	previousTarget string,
	hadPrevious bool,
	rename func(string, string) error,
) error {
	if !hadPrevious {
		if err := os.Remove(currentDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove unverified Pages current symlink: %w", err)
		}
		return nil
	}
	rollbackSymlink := currentDir + ".rollback.tmp"
	if err := os.Remove(rollbackSymlink); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(previousTarget, rollbackSymlink); err != nil {
		return err
	}
	defer func() { _ = os.Remove(rollbackSymlink) }()
	if err := rename(rollbackSymlink, currentDir); err != nil {
		return fmt.Errorf("restore previous Pages current symlink: %w", err)
	}
	gotTarget, err := os.Readlink(currentDir)
	if err != nil {
		return fmt.Errorf("verify restored Pages current symlink: %w", err)
	}
	if gotTarget != previousTarget {
		return fmt.Errorf(
			"restored Pages current symlink target %q does not match %q",
			gotTarget,
			previousTarget,
		)
	}
	return nil
}

func copyPagesDir(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil || relativePath == "." {
			return err
		}
		targetPath := filepath.Join(targetDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, pagesDirPerm)
		}
		return copyPagesFile(sourcePath, targetPath)
	})
}

func copyPagesFile(sourcePath string, targetPath string) error {
	input, err := os.Open(sourcePath) //nolint:gosec // sourcePath is under managed PagesDir walk root
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), pagesDirPerm); err != nil {
		_ = input.Close()
		return err
	}
	output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, pagesFilePerm) //nolint:gosec // targetPath is under managed PagesDir walk root
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	outputCloseErr := output.Close()
	inputCloseErr := input.Close()
	return errors.Join(copyErr, outputCloseErr, inputCloseErr)
}

func markerMatches(dir string, project pagesProjectRef) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".openflare-pages.json")) //nolint:gosec // dir is managed PagesDir
	if err != nil {
		return false
	}
	var marker pagesDeploymentMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return false
	}
	if marker.ProjectID != 0 && marker.ProjectID != project.ProjectID {
		return false
	}
	return marker.Checksum == project.Checksum
}

func writePagesMarker(dir string, project pagesProjectRef) error {
	data, err := json.Marshal(pagesDeploymentMarker(project))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".openflare-pages.json"), data, pagesManifestFilePerm)
}

func pagesProjectCurrentDir(baseDir string, projectID uint) string {
	return filepath.Join(baseDir, "projects", fmt.Sprintf("%d", projectID), "current")
}

func pagesProjectReleaseDir(baseDir string, projectID uint, checksum string) string {
	return filepath.Join(baseDir, "projects", fmt.Sprintf("%d", projectID), "releases", checksum)
}
