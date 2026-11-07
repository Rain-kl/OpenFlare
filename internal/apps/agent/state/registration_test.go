// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/localfile"
)

func TestRegistrationKeySurvivesStaleSnapshotAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	stale, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.EnsureRegistrationKey()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetRegisteredNodeID("node-server-assigned"); err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("EnsureRegistrationKey() length = %d, error %v; want 32, nil", len(decoded), err)
	}
	stale.LastError = "background sync failed"
	if err = store.Save(stale); err != nil {
		t.Fatal(err)
	}
	restartedKey, err := NewStore(path).EnsureRegistrationKey()
	if err != nil {
		t.Fatal(err)
	}
	if restartedKey != key {
		t.Error("EnsureRegistrationKey(after stale save and restart) changed enrollment identity")
	}
	snapshot, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.NodeID != "node-server-assigned" {
		t.Errorf("Load(after stale save).NodeID = %q, want server-assigned ID", snapshot.NodeID)
	}
}

func TestExistingRegistrationKeyRequiresDirectorySyncBeforeUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	key, err := store.EnsureRegistrationKey()
	if err != nil {
		t.Fatal(err)
	}
	// Model a prior successful rename whose final directory flush was not confirmed.
	syncFailure := errors.New("directory I/O failure")
	syncCalls := 0
	store.syncDirectory = func(dir string) error {
		syncCalls++
		if dir != filepath.Dir(path) {
			t.Errorf("EnsureRegistrationKey sync directory = %q, want state parent", dir)
		}
		return syncFailure
	}
	for range 2 {
		got, retryErr := store.EnsureRegistrationKey()
		if got != "" || !errors.Is(retryErr, syncFailure) {
			t.Errorf("EnsureRegistrationKey(unconfirmed directory) returned usable key or lost error: %v", retryErr)
		}
	}
	if syncCalls != 2 {
		t.Errorf("EnsureRegistrationKey(repeated failures) sync calls = %d, want 2", syncCalls)
	}
	store.syncDirectory = localfile.SyncDirectory
	confirmedKey, err := store.EnsureRegistrationKey()
	if err != nil || confirmedKey != key {
		t.Errorf("EnsureRegistrationKey(after successful sync) changed identity or failed: %v", err)
	}
}
