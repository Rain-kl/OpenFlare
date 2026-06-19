package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/agent/protocol"
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

func (s *Service) syncPagesDeployments(ctx context.Context, config *protocol.ActiveConfigResponse) error {
	deployments, err := referencedPagesDeployments(config)
	if err != nil {
		return err
	}
	if len(deployments) == 0 {
		return nil
	}
	if strings.TrimSpace(s.pagesDir) == "" {
		return errors.New("pages_dir is required when active config references Pages deployments")
	}
	for _, deployment := range deployments {
		if err := s.ensurePagesDeployment(ctx, deployment); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePagesDeployment(ctx context.Context, deployment pagesDeploymentSource) error {
	currentDir := pagesCurrentDir(s.pagesDir, deployment.DeploymentID)
	if markerMatches(currentDir, deployment) {
		return nil
	}
	packageBytes, err := s.client.DownloadPagesDeploymentPackage(ctx, deployment.DeploymentID)
	if err != nil {
		return fmt.Errorf("download Pages deployment %d: %w", deployment.DeploymentID, err)
	}
	if got := checksumBytes(packageBytes); got != deployment.Checksum {
		return fmt.Errorf("Pages deployment %d checksum mismatch: expected %s, got %s", deployment.DeploymentID, deployment.Checksum, got)
	}
	releaseDir := pagesReleaseDir(s.pagesDir, deployment.DeploymentID, deployment.Checksum)
	if !markerMatches(releaseDir, deployment) {
		if err := extractPagesPackage(packageBytes, releaseDir, deployment); err != nil {
			return err
		}
	}
	return switchPagesCurrentDir(s.pagesDir, deployment.DeploymentID, releaseDir)
}

func referencedPagesDeployments(config *protocol.ActiveConfigResponse) ([]pagesDeploymentSource, error) {
	if config == nil || strings.TrimSpace(config.SourceConfigJSON) == "" {
		return nil, nil
	}
	var doc pagesSourceDocument
	if err := json.Unmarshal([]byte(config.SourceConfigJSON), &doc); err != nil {
		return nil, fmt.Errorf("decode Pages references: %w", err)
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
			return nil, errors.New("Pages deployment snapshot is incomplete")
		}
		if _, ok := seen[deploymentID]; ok {
			continue
		}
		seen[deploymentID] = struct{}{}
		result = append(result, pagesDeploymentSource{DeploymentID: deploymentID, Checksum: checksum})
	}
	return result, nil
}

func findCommonRootPrefix(files []*zip.File) (string, error) {
	var firstFilePath string
	hasMultipleFiles := false
	for _, item := range files {
		relativePath, skip, err := normalizePagesArchivePath(item.Name)
		if err != nil {
			return "", err
		}
		if skip {
			continue
		}
		normalizedPath := filepath.ToSlash(relativePath)
		if firstFilePath == "" {
			firstFilePath = normalizedPath
		} else {
			hasMultipleFiles = true
		}
	}
	if firstFilePath == "" {
		return "", nil
	}
	parts := strings.Split(firstFilePath, "/")
	if len(parts) <= 1 {
		return "", nil
	}
	commonPrefix := parts[0] + "/"
	if hasMultipleFiles {
		for _, item := range files {
			relativePath, skip, err := normalizePagesArchivePath(item.Name)
			if err != nil {
				return "", err
			}
			if skip {
				continue
			}
			normalizedPath := filepath.ToSlash(relativePath)
			if !strings.HasPrefix(normalizedPath, commonPrefix) {
				return "", nil
			}
		}
	}
	return commonPrefix, nil
}

func extractPagesPackage(packageBytes []byte, releaseDir string, deployment pagesDeploymentSource) error {
	tmpDir := releaseDir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	reader, err := zip.NewReader(bytes.NewReader(packageBytes), int64(len(packageBytes)))
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("open Pages zip: %w", err)
	}
	commonPrefix, err := findCommonRootPrefix(reader.File)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	for _, item := range reader.File {
		relativePath, skip, err := normalizePagesArchivePath(item.Name)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
		if skip {
			continue
		}
		if commonPrefix != "" {
			slashPath := filepath.ToSlash(relativePath)
			if strings.HasPrefix(slashPath, commonPrefix) {
				relativePath = filepath.FromSlash(strings.TrimPrefix(slashPath, commonPrefix))
			}
		}
		if item.FileInfo().Mode()&os.ModeSymlink != 0 {
			_ = os.RemoveAll(tmpDir)
			return fmt.Errorf("Pages package contains unsupported symlink: %s", relativePath)
		}
		if err := extractPagesFile(item, filepath.Join(tmpDir, relativePath)); err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
	}
	if err := writePagesMarker(tmpDir, deployment); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	_ = os.RemoveAll(releaseDir)
	return os.Rename(tmpDir, releaseDir)
}

func extractPagesFile(item *zip.File, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	source, err := item.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, item.FileInfo().Mode().Perm())
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
	return err
}

func switchPagesCurrentDir(baseDir string, deploymentID uint, releaseDir string) error {
	currentDir := pagesCurrentDir(baseDir, deploymentID)
	previousDir := currentDir + ".previous"
	_ = os.RemoveAll(previousDir)
	if err := os.MkdirAll(filepath.Dir(currentDir), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(currentDir); err == nil {
		if err := os.Rename(currentDir, previousDir); err != nil {
			return err
		}
	}
	if err := copyPagesDir(releaseDir, currentDir); err != nil {
		_ = os.RemoveAll(currentDir)
		if _, restoreErr := os.Stat(previousDir); restoreErr == nil {
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
			return os.MkdirAll(targetPath, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		input, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer input.Close()
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(output, input)
		return err
	})
}

func normalizePagesArchivePath(raw string) (string, bool, error) {
	name := strings.TrimSpace(filepath.ToSlash(raw))
	if name == "" || strings.HasSuffix(name, "/") {
		return "", true, nil
	}
	if strings.HasPrefix(name, "/") {
		return "", false, fmt.Errorf("Pages package contains absolute path: %s", raw)
	}
	cleaned := path.Clean(name)
	if cleaned == "." {
		return "", true, nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", false, fmt.Errorf("Pages package path escapes deployment root: %s", raw)
	}
	return filepath.FromSlash(cleaned), false, nil
}

func markerMatches(dir string, deployment pagesDeploymentSource) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".openflare-pages.json"))
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
	data, err := json.Marshal(pagesDeploymentMarker{
		DeploymentID: deployment.DeploymentID,
		Checksum:     deployment.Checksum,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".openflare-pages.json"), data, 0o644)
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
