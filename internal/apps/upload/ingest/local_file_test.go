// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalFileFindsStoredPath(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "legacy.zip")
	if err := os.WriteFile(artifactPath, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	path, size, err := ResolveLocalFile(context.Background(), LocalFileCandidateRequest{
		StoredPath: artifactPath,
	})
	if err != nil {
		t.Fatalf("ResolveLocalFile failed: %v", err)
	}
	if path != artifactPath || size != int64(len("legacy")) {
		t.Fatalf("unexpected resolve result: path=%q size=%d", path, size)
	}
}

func TestIngestFromLocalPathRequiresPath(t *testing.T) {
	_, err := IngestFromLocalPath(context.Background(), "", Request{})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}
}
