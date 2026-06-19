package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestEnsureNodeIDPersists(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state.json"))
	nodeID1, err := store.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID failed: %v", err)
	}
	nodeID2, err := store.EnsureNodeID()
	if err != nil {
		t.Fatalf("EnsureNodeID second call failed: %v", err)
	}
	if nodeID1 == "" || nodeID1 != nodeID2 {
		t.Fatal("expected node id to persist across calls")
	}
}

func TestStore_Load_NonExistentFile(t *testing.T) {
	// Loading from a non-existent path should succeed and return an empty Snapshot
	tempFile := filepath.Join(t.TempDir(), "nonexistent.json")
	store := NewStore(tempFile)

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("expected Load to succeed for non-existent file, got err: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.NodeID != "" || snap.CurrentVersion != "" {
		t.Errorf("expected empty snapshot, got: %+v", snap)
	}
}

func TestStore_Load_EmptyFile(t *testing.T) {
	// Loading from an empty file should succeed and return an empty Snapshot
	tempFile := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(tempFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	store := NewStore(tempFile)
	snap, err := store.Load()
	if err != nil {
		t.Fatalf("expected Load to succeed for empty file, got err: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.NodeID != "" {
		t.Errorf("expected empty snapshot, got: %+v", snap)
	}
}

func TestStore_Load_InvalidJSON(t *testing.T) {
	// Loading from a corrupted file with invalid JSON should fail with parsing error
	tempFile := filepath.Join(t.TempDir(), "corrupted.json")
	if err := os.WriteFile(tempFile, []byte("{invalid-json"), 0644); err != nil {
		t.Fatalf("failed to create corrupted file: %v", err)
	}

	store := NewStore(tempFile)
	_, err := store.Load()
	if err == nil {
		t.Fatal("expected Load to fail for corrupted JSON file")
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(tempFile)

	original := &Snapshot{
		NodeID:          "node-test-123",
		CurrentVersion:  "20260531-001",
		CurrentChecksum: "chk-active-xyz",
		BlockedVersion:  "20260531-002",
		BlockedChecksum: "chk-blocked-abc",
		BlockedReason:   "invalid upstream domain name",
		LastError:       "configuration reload timeout",
		OpenrestyStatus: "unhealthy",
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("expected Save to succeed, got: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("expected Load to succeed, got: %v", err)
	}

	if loaded.NodeID != original.NodeID ||
		loaded.CurrentVersion != original.CurrentVersion ||
		loaded.CurrentChecksum != original.CurrentChecksum ||
		loaded.BlockedVersion != original.BlockedVersion ||
		loaded.BlockedChecksum != original.BlockedChecksum ||
		loaded.BlockedReason != original.BlockedReason ||
		loaded.LastError != original.LastError ||
		loaded.OpenrestyStatus != original.OpenrestyStatus {
		t.Errorf("loaded snapshot does not match original: %+v vs %+v", loaded, original)
	}
}

func TestStore_ConcurrencySafety(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(tempFile)

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	// Run concurrent writers and readers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Concurrently save
				snap := &Snapshot{
					NodeID:         fmt.Sprintf("node-%d", workerID),
					CurrentVersion: fmt.Sprintf("v-%d", j),
				}
				if err := store.Save(snap); err != nil {
					t.Errorf("Save failed under concurrency: %v", err)
				}

				// Concurrently load
				if _, err := store.Load(); err != nil {
					t.Errorf("Load failed under concurrency: %v", err)
				}

				// Concurrently ensure ID
				if _, err := store.EnsureNodeID(); err != nil {
					t.Errorf("EnsureNodeID failed under concurrency: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}
