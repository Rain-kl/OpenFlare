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
)

type pagesSourceDocument struct {
	Routes []pagesSourceRoute `json:"routes"`
}

type pagesSourceRoute struct {
	UpstreamType    string                 `json:"upstream_type"`
	PagesDeployment *pagesDeploymentSource `json:"pages_deployment"`
}

type pagesDeploymentSource struct {
	DeploymentID uint   `json:"deployment_id"`
	Checksum     string `json:"checksum"`
}

type pagesDeploymentMarker struct {
	DeploymentID uint   `json:"deployment_id"`
	Checksum     string `json:"checksum"`
}

func pagesDeploymentStateHash(item state.PagesDeployment) string {
	if hash := strings.TrimSpace(item.Hash); hash != "" {
		return hash
	}
	return strings.TrimSpace(item.Checksum)
}

func snapshotPagesDeployments(snapshot *state.Snapshot) []pagesDeploymentSource {
	if snapshot == nil || snapshot.PagesDeployments == nil {
		return nil
	}
	result := make([]pagesDeploymentSource, 0, len(snapshot.PagesDeployments))
	for _, item := range snapshot.PagesDeployments {
		result = append(result, pagesDeploymentSource{
			DeploymentID: item.DeploymentID,
			Checksum:     pagesDeploymentStateHash(item),
		})
	}
	return result
}

func setSnapshotPagesDeployments(snapshot *state.Snapshot, deployments []pagesDeploymentSource) {
	if snapshot == nil {
		return
	}
	if len(deployments) == 0 {
		snapshot.PagesDeployments = []state.PagesDeployment{}
		return
	}
	snapshot.PagesDeployments = make([]state.PagesDeployment, len(deployments))
	for i, deployment := range deployments {
		snapshot.PagesDeployments[i] = state.PagesDeployment{
			DeploymentID: deployment.DeploymentID,
			Hash:         strings.TrimSpace(deployment.Checksum),
		}
	}
}

func updateSnapshotPagesDeploymentHash(snapshot *state.Snapshot, deployment pagesDeploymentSource) {
	if snapshot == nil || snapshot.PagesDeployments == nil {
		return
	}
	hash := strings.TrimSpace(deployment.Checksum)
	for i := range snapshot.PagesDeployments {
		if snapshot.PagesDeployments[i].DeploymentID != deployment.DeploymentID {
			continue
		}
		snapshot.PagesDeployments[i].Hash = hash
		snapshot.PagesDeployments[i].Checksum = ""
		return
	}
}

func pagesDiscoveryNeeded(snapshot *state.Snapshot) bool {
	return snapshot == nil || snapshot.PagesDeployments == nil
}

func pagesSyncNeeded(snapshot *state.Snapshot) bool {
	return snapshot != nil && snapshot.PagesDeployments != nil && len(snapshot.PagesDeployments) > 0
}

func pagesReconcileNeeded(snapshot *state.Snapshot) bool {
	if pagesDiscoveryNeeded(snapshot) {
		return true
	}
	return pagesSyncNeeded(snapshot)
}

func (s *Service) syncPagesDeployments(ctx context.Context, snapshot *state.Snapshot, config *protocol.ActiveConfigResponse) error {
	var deployments []pagesDeploymentSource
	var err error
	if config != nil {
		deployments, err = referencedPagesDeployments(config)
		if err != nil {
			return err
		}
		setSnapshotPagesDeployments(snapshot, deployments)
	} else {
		deployments = snapshotPagesDeployments(snapshot)
	}
	if len(deployments) == 0 {
		return nil
	}
	if strings.TrimSpace(s.pagesDir) == "" {
		return errors.New("pages_dir is required when active config references Pages deployments")
	}
	for _, deployment := range deployments {
		if err := s.ensurePagesDeployment(ctx, snapshot, deployment); err != nil {
			return err
		}
	}
	if s.nginxManager != nil {
		if err := s.nginxManager.EnsureWorkerReadAccess(); err != nil {
			return fmt.Errorf("ensure openresty worker read access: %w", err)
		}
	}
	return nil
}

func (s *Service) ensurePagesDeployment(ctx context.Context, snapshot *state.Snapshot, deployment pagesDeploymentSource) error {
	serverHash, err := s.client.GetPagesDeploymentHash(ctx, deployment.DeploymentID)
	if err != nil {
		return fmt.Errorf("fetch Pages deployment %d hash: %w", deployment.DeploymentID, err)
	}
	serverHash = strings.TrimSpace(serverHash)
	if serverHash == "" {
		return fmt.Errorf("pages deployment %d hash is empty", deployment.DeploymentID)
	}
	effective := pagesDeploymentSource{
		DeploymentID: deployment.DeploymentID,
		Checksum:     serverHash,
	}
	updateSnapshotPagesDeploymentHash(snapshot, effective)

	releaseDir := pagesReleaseDir(s.pagesDir, effective.DeploymentID, effective.Checksum)
	if pagesReleaseReady(releaseDir, effective) {
		return switchPagesCurrentDir(s.pagesDir, effective.DeploymentID, releaseDir)
	}
	packageBytes, err := s.client.DownloadPagesDeploymentPackage(ctx, effective.DeploymentID)
	if err != nil {
		return fmt.Errorf("download Pages deployment %d: %w", effective.DeploymentID, err)
	}
	if got := checksumBytes(packageBytes); got != effective.Checksum {
		return fmt.Errorf("pages deployment %d checksum mismatch: expected %s, got %s", effective.DeploymentID, effective.Checksum, got)
	}
	if err := extractPagesPackage(packageBytes, releaseDir, effective); err != nil {
		return err
	}
	return switchPagesCurrentDir(s.pagesDir, effective.DeploymentID, releaseDir)
}

func pagesReleaseReady(dir string, deployment pagesDeploymentSource) bool {
	if !markerMatches(dir, deployment) {
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

func referencedPagesDeployments(config *protocol.ActiveConfigResponse) ([]pagesDeploymentSource, error) {
	if config == nil || strings.TrimSpace(config.SourceConfigJSON) == "" {
		return nil, nil
	}
	var doc pagesSourceDocument
	if err := json.Unmarshal([]byte(config.SourceConfigJSON), &doc); err != nil {
		return nil, fmt.Errorf("decode pages references: %w", err)
	}
	seen := make(map[uint]struct{})
	result := make([]pagesDeploymentSource, 0)
	for _, route := range doc.Routes {
		if strings.ToLower(strings.TrimSpace(route.UpstreamType)) != "pages" || route.PagesDeployment == nil {
			continue
		}
		deploymentID := route.PagesDeployment.DeploymentID
		checksum := strings.TrimSpace(route.PagesDeployment.Checksum)
		if deploymentID == 0 || checksum == "" {
			return nil, errors.New("pages deployment snapshot is incomplete")
		}
		if _, ok := seen[deploymentID]; ok {
			continue
		}
		seen[deploymentID] = struct{}{}
		result = append(result, pagesDeploymentSource{DeploymentID: deploymentID, Checksum: checksum})
	}
	return result, nil
}

func extractPagesPackage(packageBytes []byte, releaseDir string, deployment pagesDeploymentSource) error {
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
	// Control plane already inspected and accepted this package. Agent only
	// verifies download integrity (checksum) and performs local-safe extract
	// (path escape / symlink guards). Size and file-count limits are not re-applied.
	if err := pagesarchive.ExtractBytes(packageBytes, format, tmpDir, pagesarchive.ExtractOptions{
		StripCommonRoot: true,
		EnforceLimits:   false,
	}); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("extract Pages package: %w", err)
	}
	if err := writePagesMarker(tmpDir, deployment); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	_ = os.RemoveAll(releaseDir)
	return os.Rename(tmpDir, releaseDir)
}

func switchPagesCurrentDir(baseDir string, deploymentID uint, releaseDir string) error {
	currentDir := pagesCurrentDir(baseDir, deploymentID)
	previousDir := currentDir + ".previous"
	_ = os.RemoveAll(previousDir)
	if err := os.MkdirAll(filepath.Dir(currentDir), pagesDirPerm); err != nil {
		return err
	}

	relTarget, err := filepath.Rel(filepath.Dir(currentDir), releaseDir)
	if err != nil {
		relTarget = releaseDir
	}

	// Try creating a temporary symlink first to check if symlinks are supported/feasible
	tmpSymlink := currentDir + ".tmp"
	_ = os.Remove(tmpSymlink)

	symlinkErr := os.Symlink(relTarget, tmpSymlink)
	if symlinkErr != nil {
		return fallbackCopyPagesCurrentDir(currentDir, previousDir, releaseDir)
	}

	// Symlink is supported, proceed with symlink swap
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

func markerMatches(dir string, deployment pagesDeploymentSource) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".openflare-pages.json")) //nolint:gosec // dir is managed PagesDir
	if err != nil {
		return false
	}
	var marker pagesDeploymentMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return false
	}
	return marker.DeploymentID == deployment.DeploymentID && marker.Checksum == deployment.Checksum
}

func writePagesMarker(dir string, deployment pagesDeploymentSource) error {
	data, err := json.Marshal(pagesDeploymentMarker(deployment))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".openflare-pages.json"), data, pagesManifestFilePerm)
}

func pagesCurrentDir(baseDir string, deploymentID uint) string {
	return filepath.Join(baseDir, "deployments", fmt.Sprintf("%d", deploymentID), "current")
}

func pagesReleaseDir(baseDir string, deploymentID uint, checksum string) string {
	return filepath.Join(baseDir, "deployments", fmt.Sprintf("%d", deploymentID), "releases", checksum)
}

func checksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
