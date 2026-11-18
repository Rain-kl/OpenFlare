// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package localfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicReplacesCompleteFileAndCleansFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.json")
	if err := WriteAtomic(path, []byte("old identity"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("new identity"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new identity" {
		t.Errorf("WriteAtomic replacement = %q, %v; want complete new identity", data, err)
	}
	blocked := filepath.Join(dir, "directory")
	if err = os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = WriteAtomic(blocked, []byte("cannot replace directory"), 0o600); err == nil {
		t.Fatal("WriteAtomic(directory) succeeded, want rename failure")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("WriteAtomic(after failure) directory entries = %d, want 2 with no temporary file", len(entries))
	}
}

func TestWriteAtomicReportsDirectorySyncFailureAfterReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.json")
	if err := os.WriteFile(path, []byte("old identity"), 0o600); err != nil {
		t.Fatal(err)
	}
	syncFailure := errors.New("directory I/O failure")
	syncCalls := 0
	err := writeAtomic(path, []byte("new identity"), 0o600, func(gotDir string) error {
		syncCalls++
		if gotDir != dir {
			t.Errorf("writeAtomic sync path = %q, want %q", gotDir, dir)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || string(data) != "new identity" {
			t.Errorf("Directory sync ran before replacement: contents = %q, error %v", data, readErr)
		}
		return syncFailure
	})
	if !errors.Is(err, syncFailure) || syncCalls != 1 {
		t.Errorf("writeAtomic(directory sync failure) = %v, %d sync calls; want I/O failure, 1", err, syncCalls)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("writeAtomic(failed directory sync) left %d entries, want only destination", len(entries))
	}
}

func TestSyncDirectoryRejectsMissingAndNonDirectoryPaths(t *testing.T) {
	dir := t.TempDir()
	if err := SyncDirectory(dir); err != nil {
		t.Fatalf("SyncDirectory(existing directory) = %v, want nil", err)
	}
	if err := SyncDirectory(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("SyncDirectory(missing directory) = %v, want not-exist error", err)
	}
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncDirectory(path); err == nil {
		t.Error("SyncDirectory(regular file) = nil, want non-directory error")
	}
}
