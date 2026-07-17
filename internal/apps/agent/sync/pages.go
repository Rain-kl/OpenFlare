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
	pagesDirPerm          = 0o755
	pagesFilePerm         = 0o644
	pagesManifestFilePerm = 0o644
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
		hash := strings.TrimSpace(latest.Hash)
		if hash == "" {
			return fmt.Errorf("pages project %d latest hash is empty", projectID)
		}
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

		packageBytes, err := s.client.DownloadPagesProjectLatestPackage(ctx, projectID)
		if err != nil {
			return fmt.Errorf("download Pages project %d latest package: %w", projectID, err)
		}
		got := checksumBytes(packageBytes)

		// Re-probe latest after download to detect activation races.
		// Accept the package only when its content hash still matches latest.
		verify, err := s.client.GetPagesProjectLatestHash(ctx, projectID)
		if err != nil {
			return fmt.Errorf("re-fetch Pages project %d latest hash: %w", projectID, err)
		}
		verifyHash := strings.TrimSpace(verify.Hash)
		if verifyHash == "" {
			return fmt.Errorf("pages project %d latest hash is empty", projectID)
		}
		if got != verifyHash {
			lastErr = fmt.Errorf(
				"pages project %d package/hash race: downloaded %s, latest now %s (attempt %d/%d)",
				projectID, got, verifyHash, attempt+1, pagesLatestPullAttempts,
			)
			slog.Warn("pages latest package race, retrying",
				"project_id", projectID,
				"downloaded_hash", got,
				"latest_hash", verifyHash,
				"attempt", attempt+1,
			)
			continue
		}

		effective = pagesProjectRef{
			ProjectID:    projectID,
			DeploymentID: verify.DeploymentID,
			Checksum:     got,
		}
		releaseDir = pagesProjectReleaseDir(s.pagesDir, projectID, got)
		if err := extractPagesPackage(packageBytes, releaseDir, effective); err != nil {
			return err
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

func extractPagesPackage(packageBytes []byte, releaseDir string, project pagesProjectRef) error {
	tmpDir := releaseDir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, pagesDirPerm); err != nil {
		return err
	}
	format, err := pagesarchive.DetectFormat("", packageBytes)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("detect Pages package format: %w", err)
	}
	// Control plane already inspected and accepted this package.
	if err := pagesarchive.ExtractBytes(packageBytes, format, tmpDir, pagesarchive.ExtractOptions{
		StripCommonRoot: true,
		EnforceLimits:   false,
	}); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("extract Pages package: %w", err)
	}
	if err := writePagesMarker(tmpDir, project); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	_ = os.RemoveAll(releaseDir)
	return os.Rename(tmpDir, releaseDir)
}

func switchPagesProjectCurrentDir(baseDir string, projectID uint, releaseDir string) error {
	currentDir := pagesProjectCurrentDir(baseDir, projectID)
	previousDir := currentDir + ".previous"
	_ = os.RemoveAll(previousDir)
	if err := os.MkdirAll(filepath.Dir(currentDir), pagesDirPerm); err != nil {
		return err
	}

	relTarget, err := filepath.Rel(filepath.Dir(currentDir), releaseDir)
	if err != nil {
		relTarget = releaseDir
	}

	tmpSymlink := currentDir + ".tmp"
	_ = os.Remove(tmpSymlink)

	symlinkErr := os.Symlink(relTarget, tmpSymlink)
	if symlinkErr != nil {
		return fallbackCopyPagesCurrentDir(currentDir, previousDir, releaseDir)
	}
	_ = os.Remove(tmpSymlink)

	if _, err := os.Lstat(currentDir); err == nil {
		if err := os.Rename(currentDir, previousDir); err != nil {
			return err
		}
	}

	if err := os.Symlink(relTarget, currentDir); err != nil {
		if _, restoreErr := os.Lstat(previousDir); restoreErr == nil {
			_ = os.Rename(previousDir, currentDir)
		}
		return err
	}
	_ = os.RemoveAll(previousDir)
	return nil
}

func fallbackCopyPagesCurrentDir(currentDir, previousDir, releaseDir string) error {
	if _, err := os.Lstat(currentDir); err == nil {
		if err := os.Rename(currentDir, previousDir); err != nil {
			return err
		}
	}
	if err := copyPagesDir(releaseDir, currentDir); err != nil {
		_ = os.RemoveAll(currentDir)
		if _, restoreErr := os.Lstat(previousDir); restoreErr == nil {
			_ = os.Rename(previousDir, currentDir)
		}
		return err
	}
	_ = os.RemoveAll(previousDir)
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
		input, err := os.Open(sourcePath) //nolint:gosec // sourcePath is under managed PagesDir walk root
		if err != nil {
			return err
		}
		defer func() { _ = input.Close() }()
		if err := os.MkdirAll(filepath.Dir(targetPath), pagesDirPerm); err != nil {
			return err
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, pagesFilePerm) //nolint:gosec // targetPath is under managed PagesDir walk root
		if err != nil {
			return err
		}
		defer func() { _ = output.Close() }()
		_, err = io.Copy(output, input)
		return err
	})
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

func checksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
