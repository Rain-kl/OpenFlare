// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
)

func TestEnsurePagesProjectRejectsMetadataBeyondAgentCapsBeforeDownload(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{"index.html": "x"})
	base := protocol.PagesProjectLatestHashResponse{
		ProjectID:    1,
		DeploymentID: 1,
		Hash:         testBytesChecksum(packageBytes),
		PackageSize:  int64(len(packageBytes)),
		FileCount:    1,
		TotalSize:    1,
	}
	tests := []struct {
		name   string
		mutate func(*protocol.PagesProjectLatestHashResponse)
	}{
		{
			name: "package size",
			mutate: func(metadata *protocol.PagesProjectLatestHashResponse) {
				metadata.PackageSize = agentPagesMaxPackageBytes + 1
			},
		},
		{
			name: "file count",
			mutate: func(metadata *protocol.PagesProjectLatestHashResponse) {
				metadata.FileCount = agentPagesMaxFiles + 1
			},
		},
		{
			name: "total size",
			mutate: func(metadata *protocol.PagesProjectLatestHashResponse) {
				metadata.TotalSize = agentPagesMaxTotalBytes + 1
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := base
			test.mutate(&metadata)
			client := &fakeClient{
				pagesPackages: map[uint][]byte{1: packageBytes},
				pagesMetadata: map[uint]protocol.PagesProjectLatestHashResponse{1: metadata},
			}
			service := New(client, &fakeManager{}, nil)
			service.SetPagesDir(t.TempDir())
			err := service.ensurePagesProject(context.Background(), &state.Snapshot{}, 1)
			if err == nil || !strings.Contains(err.Error(), "agent limit") {
				t.Fatalf("ensurePagesProject(%s metadata) error = %v, want agent limit error", test.name, err)
			}
			if client.pagesPackageDownloads != 0 {
				t.Errorf("ensurePagesProject(%s metadata) downloads = %d, want 0", test.name, client.pagesPackageDownloads)
			}
		})
	}
}

func TestEnsurePagesProjectRetriesSameHashDifferentDeployment(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{"index.html": "same"})
	hash := testBytesChecksum(packageBytes)
	client := &racingLatestClient{
		pkgA:  packageBytes,
		pkgB:  packageBytes,
		hashA: hash,
		hashB: hash,
	}
	service := New(client, &fakeManager{}, nil)
	pagesDir := t.TempDir()
	service.SetPagesDir(pagesDir)
	snapshot := &state.Snapshot{PagesDeployments: []state.PagesDeployment{{ProjectID: 42}}}

	if err := service.ensurePagesProject(context.Background(), snapshot, 42); err != nil {
		t.Fatalf("ensurePagesProject(same hash deployment race) error = %v", err)
	}
	if client.downloadCalls != 2 {
		t.Errorf("ensurePagesProject(same hash deployment race) downloads = %d, want 2", client.downloadCalls)
	}
	if snapshot.PagesDeployments[0].DeploymentID != 2 || snapshot.PagesDeployments[0].Hash != hash {
		t.Errorf("snapshot Pages deployment = %+v, want deployment 2/hash %s", snapshot.PagesDeployments[0], hash)
	}
}

func TestEnsurePagesProjectExtractionFailureCleansTempAndPreservesCurrent(t *testing.T) {
	projectID := uint(9)
	oldPackage := testPagesPackage(t, map[string]string{"index.html": "old"})
	oldHash := testBytesChecksum(oldPackage)
	newPackage := testPagesPackage(t, map[string]string{"index.html": "new"})
	newHash := testBytesChecksum(newPackage)
	pagesDir := t.TempDir()
	oldRelease := pagesProjectReleaseDir(pagesDir, projectID, oldHash)
	if err := extractTestPagesPackage(t, oldPackage, oldRelease, pagesProjectRef{
		ProjectID:    projectID,
		DeploymentID: 1,
		Checksum:     oldHash,
	}); err != nil {
		t.Fatalf("extractTestPagesPackage(old) error = %v", err)
	}
	if err := switchPagesProjectCurrentDir(pagesDir, projectID, oldRelease); err != nil {
		t.Fatalf("switchPagesProjectCurrentDir(old) error = %v", err)
	}

	client := &fakeClient{
		pagesPackages: map[uint][]byte{projectID: newPackage},
		pagesMetadata: map[uint]protocol.PagesProjectLatestHashResponse{
			projectID: {
				ProjectID:    projectID,
				DeploymentID: 2,
				Hash:         newHash,
				PackageSize:  int64(len(newPackage)),
				FileCount:    1,
				TotalSize:    2, // Smaller than the actual three-byte file.
			},
		},
	}
	service := New(client, &fakeManager{}, nil)
	service.SetPagesDir(pagesDir)
	err := service.ensurePagesProject(context.Background(), &state.Snapshot{}, projectID)
	if err == nil {
		t.Fatal("ensurePagesProject(metadata-tightened extraction) error = nil, want error")
	}
	current, readErr := os.ReadFile(pagesProjectCurrentDir(pagesDir, projectID) + "/index.html")
	if readErr != nil {
		t.Fatalf("read old current after failed extraction error = %v", readErr)
	}
	if string(current) != "old" {
		t.Errorf("current content after failed extraction = %q, want %q", current, "old")
	}
	entries, readErr := os.ReadDir(filepath.Join(pagesDir, "projects", "9", "releases"))
	if readErr != nil {
		t.Fatalf("read releases after failed extraction error = %v", readErr)
	}
	if len(entries) != 1 || entries[0].Name() != oldHash {
		t.Errorf("releases after failed extraction = %v, want only %s", entries, oldHash)
	}
}

func TestEnsurePagesProjectAcceptsAllZeroByteFiles(t *testing.T) {
	packageBytes := testPagesPackage(t, map[string]string{
		"index.html": "",
		".gitkeep":   "",
	})
	client := &fakeClient{pagesPackages: map[uint][]byte{5: packageBytes}}
	service := New(client, &fakeManager{}, nil)
	pagesDir := t.TempDir()
	service.SetPagesDir(pagesDir)

	if err := service.ensurePagesProject(context.Background(), &state.Snapshot{}, 5); err != nil {
		t.Fatalf("ensurePagesProject(all-zero files) error = %v", err)
	}
	for _, name := range []string{"index.html", ".gitkeep"} {
		info, err := os.Stat(filepath.Join(pagesProjectCurrentDir(pagesDir, 5), name))
		if err != nil {
			t.Errorf("stat all-zero file %q error = %v", name, err)
			continue
		}
		if info.Size() != 0 {
			t.Errorf("all-zero file %q size = %d, want 0", name, info.Size())
		}
	}
}

func TestSwitchPagesProjectCurrentDirRenameFailureKeepsPreviousCurrent(t *testing.T) {
	pagesDir := t.TempDir()
	projectID := uint(21)
	oldRelease := pagesProjectReleaseDir(pagesDir, projectID, "old")
	newRelease := pagesProjectReleaseDir(pagesDir, projectID, "new")
	for path, content := range map[string]string{
		oldRelease: "old",
		newRelease: "new",
	} {
		if err := os.MkdirAll(path, pagesDirPerm); err != nil {
			t.Fatalf("mkdir release %q error = %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(path, "index.html"), []byte(content), pagesFilePerm); err != nil {
			t.Fatalf("write release %q error = %v", path, err)
		}
	}
	if err := switchPagesProjectCurrentDir(pagesDir, projectID, oldRelease); err != nil {
		t.Fatalf("seed previous current error = %v", err)
	}
	currentDir := pagesProjectCurrentDir(pagesDir, projectID)
	renameErr := errors.New("injected current rename failure")
	err := switchPagesProjectCurrentDirWithRename(
		pagesDir,
		projectID,
		newRelease,
		func(oldPath string, newPath string) error {
			if oldPath == currentDir+".tmp" && newPath == currentDir {
				return renameErr
			}
			return os.Rename(oldPath, newPath)
		},
	)
	if !errors.Is(err, renameErr) {
		t.Fatalf("switchPagesProjectCurrentDirWithRename() error = %v, want injected rename error", err)
	}
	current, err := os.ReadFile(filepath.Join(currentDir, "index.html"))
	if err != nil {
		t.Fatalf("read previous current after rename failure error = %v", err)
	}
	if string(current) != "old" {
		t.Errorf("current after rename failure = %q, want %q", current, "old")
	}
	if _, err := os.Lstat(currentDir + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary current symlink remains after rename failure: %v", err)
	}
}

func TestPromoteSameHashReleaseFailureRestoresPreviousCurrent(t *testing.T) {
	pagesDir := t.TempDir()
	projectID := uint(22)
	hash := "same-hash"
	project := pagesProjectRef{ProjectID: projectID, DeploymentID: 2, Checksum: hash}
	releaseDir := pagesProjectReleaseDir(pagesDir, projectID, hash)
	if err := os.MkdirAll(releaseDir, pagesDirPerm); err != nil {
		t.Fatalf("mkdir previous same-hash release error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "index.html"), []byte("old"), pagesFilePerm); err != nil {
		t.Fatalf("write previous same-hash release error = %v", err)
	}
	if err := writePagesMarker(releaseDir, project); err != nil {
		t.Fatalf("write previous same-hash marker error = %v", err)
	}
	if err := switchPagesProjectCurrentDir(pagesDir, projectID, releaseDir); err != nil {
		t.Fatalf("seed same-hash current error = %v", err)
	}
	stagingDir, err := os.MkdirTemp(filepath.Dir(releaseDir), ".same-hash-*.tmp")
	if err != nil {
		t.Fatalf("create same-hash staging error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "index.html"), []byte("new"), pagesFilePerm); err != nil {
		t.Fatalf("write repaired same-hash release error = %v", err)
	}
	if err := writePagesMarker(stagingDir, project); err != nil {
		t.Fatalf("write repaired same-hash marker error = %v", err)
	}
	copyErr := errors.New("injected same-hash copy failure")
	err = promotePagesReleaseWithCopy(
		stagingDir,
		releaseDir,
		project,
		func(_ string, targetDir string) error {
			if err := os.MkdirAll(targetDir, pagesDirPerm); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(targetDir, "index.html"), []byte("partial"), pagesFilePerm); err != nil {
				return err
			}
			return copyErr
		},
	)
	if !errors.Is(err, copyErr) {
		t.Fatalf("promotePagesReleaseWithCopy() error = %v, want injected copy error", err)
	}
	for name, path := range map[string]string{
		"current": filepath.Join(pagesProjectCurrentDir(pagesDir, projectID), "index.html"),
		"release": filepath.Join(releaseDir, "index.html"),
	} {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read restored %s after same-hash repair failure error = %v", name, readErr)
		}
		if string(content) != "old" {
			t.Errorf("restored %s after same-hash repair failure = %q, want %q", name, content, "old")
		}
	}
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		t.Errorf("same-hash staging remains after successful rollback: %v", err)
	}
}

func TestPromotePagesReleaseRepairsDanglingCurrent(t *testing.T) {
	pagesDir := t.TempDir()
	projectID := uint(23)
	releaseDir := pagesProjectReleaseDir(pagesDir, projectID, "new-hash")
	currentDir := pagesProjectCurrentDir(pagesDir, projectID)
	requireTestMkdirAll(t, filepath.Dir(currentDir))
	relTarget, err := filepath.Rel(filepath.Dir(currentDir), releaseDir)
	if err != nil {
		t.Fatalf("relative release target error = %v", err)
	}
	if err := os.Symlink(relTarget, currentDir); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	requireTestMkdirAll(t, filepath.Dir(releaseDir))
	stagingDir, err := os.MkdirTemp(filepath.Dir(releaseDir), ".dangling-*.tmp")
	if err != nil {
		t.Fatalf("create dangling repair staging error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "index.html"), []byte("repaired"), pagesFilePerm); err != nil {
		t.Fatalf("write dangling repair staging error = %v", err)
	}
	project := pagesProjectRef{ProjectID: projectID, DeploymentID: 1, Checksum: "new-hash"}
	if err := writePagesMarker(stagingDir, project); err != nil {
		t.Fatalf("write dangling repair marker error = %v", err)
	}

	if err := promotePagesRelease(stagingDir, releaseDir, project); err != nil {
		t.Fatalf("promotePagesRelease(dangling current) error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(currentDir, "index.html"))
	if err != nil {
		t.Fatalf("read repaired dangling current error = %v", err)
	}
	if string(content) != "repaired" {
		t.Errorf("repaired dangling current = %q, want %q", content, "repaired")
	}
}

func TestSwitchPagesCurrentDirCopiesOverLegacyDirectory(t *testing.T) {
	pagesDir := t.TempDir()
	currentDir := filepath.Join(pagesDir, "current")
	releaseDir := filepath.Join(pagesDir, "releases", "new")
	requireTestMkdirAll(t, currentDir)
	requireTestMkdirAll(t, releaseDir)
	if err := os.WriteFile(filepath.Join(currentDir, "index.html"), []byte("old"), pagesFilePerm); err != nil {
		t.Fatalf("write legacy current error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "index.html"), []byte("new"), pagesFilePerm); err != nil {
		t.Fatalf("write new release error = %v", err)
	}

	if err := switchPagesCurrentDir(currentDir, releaseDir, os.Rename); err != nil {
		t.Fatalf("switchPagesCurrentDir(legacy directory) error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(currentDir, "index.html"))
	if err != nil {
		t.Fatalf("read copied legacy current error = %v", err)
	}
	if string(content) != "new" {
		t.Errorf("copied legacy current = %q, want %q", content, "new")
	}
}

func TestSwitchPagesCurrentDirFallsBackWhenSymlinkUnavailable(t *testing.T) {
	pagesDir := t.TempDir()
	currentDir := filepath.Join(pagesDir, "current")
	releaseDir := filepath.Join(pagesDir, "releases", "new")
	requireTestMkdirAll(t, releaseDir)
	if err := os.WriteFile(filepath.Join(releaseDir, "index.html"), []byte("new"), pagesFilePerm); err != nil {
		t.Fatalf("write fallback release error = %v", err)
	}
	symlinkErr := errors.New("injected symlink unavailable")
	if err := switchPagesCurrentDirWithOps(
		currentDir,
		releaseDir,
		os.Rename,
		func(string, string) error { return symlinkErr },
	); err != nil {
		t.Fatalf("switchPagesCurrentDirWithOps(symlink unavailable) error = %v", err)
	}
	info, err := os.Lstat(currentDir)
	if err != nil {
		t.Fatalf("lstat copied current error = %v", err)
	}
	if !info.IsDir() {
		t.Errorf("copied current mode = %v, want directory", info.Mode())
	}
	content, err := os.ReadFile(filepath.Join(currentDir, "index.html"))
	if err != nil {
		t.Fatalf("read fallback current error = %v", err)
	}
	if string(content) != "new" {
		t.Errorf("fallback current = %q, want %q", content, "new")
	}
}

func requireTestMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, pagesDirPerm); err != nil {
		t.Fatalf("mkdir %q error = %v", dir, err)
	}
}
