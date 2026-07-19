// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func TestSourceLeaseHeartbeatRenewsAndCancelsOnOwnershipLoss(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "lease-heartbeat")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip",
		false,
	)
	snapshot, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil || outcome != sourceLeaseAcquired || snapshot == nil {
		t.Fatalf("acquireSourceLease(heartbeat) = (%+v, %q, %v), want acquired", snapshot, outcome, err)
	}

	workCtx, heartbeat, err := startSourceLeaseHeartbeat(ctx, snapshot, 500*time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("startSourceLeaseHeartbeat() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = heartbeat.stop() })

	var initial model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&initial).Error; err != nil {
		t.Fatalf("load initial heartbeat runtime error = %v, want nil", err)
	}
	if initial.LeaseExpiresAt == nil {
		t.Fatal("initial heartbeat expiry = nil, want non-nil")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var renewedRuntime model.PagesProjectSourceRuntime
		if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&renewedRuntime).Error; err != nil {
			t.Fatalf("load renewed heartbeat runtime error = %v, want nil", err)
		}
		if renewedRuntime.LeaseExpiresAt != nil && renewedRuntime.LeaseExpiresAt.After(*initial.LeaseExpiresAt) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("heartbeat did not extend lease before deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", source.ID).
		Update("lease_token", "replacement-owner").Error; err != nil {
		t.Fatalf("replace heartbeat lease owner error = %v, want nil", err)
	}
	select {
	case <-workCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat work context was not canceled after ownership loss")
	}
	if err := heartbeat.stop(); !errors.Is(err, errSourceLeaseHeartbeatLost) {
		t.Fatalf("heartbeat.stop() error = %v, want %v", err, errSourceLeaseHeartbeatLost)
	}
	if err := heartbeat.stop(); !errors.Is(err, errSourceLeaseHeartbeatLost) {
		t.Fatalf("heartbeat.stop() second error = %v, want stable %v", err, errSourceLeaseHeartbeatLost)
	}
}

func TestAcquireSourceLeaseConcurrentOnlyOneOwner(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "lease-concurrent")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip",
		false,
	)
	type leaseResult struct {
		snapshot *sourceExecutionSnapshot
		outcome  sourceLeaseOutcome
		err      error
	}
	results := make(chan leaseResult, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			snapshot, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
			results <- leaseResult{snapshot: snapshot, outcome: outcome, err: err}
		}()
	}
	workers.Wait()
	close(results)

	acquired := 0
	busy := 0
	for result := range results {
		if result.err != nil {
			t.Errorf("acquireSourceLease(concurrent) error = %v, want nil", result.err)
			continue
		}
		switch result.outcome {
		case sourceLeaseAcquired:
			acquired++
			if result.snapshot == nil || result.snapshot.LeaseToken == "" {
				t.Errorf("acquireSourceLease(concurrent acquired) snapshot = %+v, want token-bearing snapshot", result.snapshot)
			}
		case sourceLeaseBusy:
			busy++
			if result.snapshot != nil {
				t.Errorf("acquireSourceLease(concurrent busy) snapshot = %+v, want nil", result.snapshot)
			}
		default:
			t.Errorf("acquireSourceLease(concurrent) outcome = %q, want acquired or busy", result.outcome)
		}
	}
	if acquired != 1 || busy != 1 {
		t.Errorf("concurrent lease outcomes = acquired:%d busy:%d, want 1 and 1", acquired, busy)
	}
}

func TestAcquireSourceLeaseMutualExclusionExpiryAndTerminalOwnership(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "lease-cas")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip",
		false,
	)

	first, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil {
		t.Fatalf("acquireSourceLease(first) error = %v, want nil", err)
	}
	if got, want := outcome, sourceLeaseAcquired; got != want {
		t.Fatalf("acquireSourceLease(first) outcome = %q, want %q", got, want)
	}
	if first == nil || first.LeaseToken == "" {
		t.Fatalf("acquireSourceLease(first) snapshot = %+v, want token-bearing snapshot", first)
	}

	second, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil {
		t.Fatalf("acquireSourceLease(duplicate) error = %v, want nil", err)
	}
	if got, want := outcome, sourceLeaseBusy; got != want {
		t.Errorf("acquireSourceLease(duplicate) outcome = %q, want %q", got, want)
	}
	if second != nil {
		t.Errorf("acquireSourceLease(duplicate) snapshot = %+v, want nil", second)
	}

	past := time.Now().Add(-time.Second)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", source.ID).
		Update("lease_expires_at", &past).Error; err != nil {
		t.Fatalf("expire first lease error = %v, want nil", err)
	}
	takeover, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil {
		t.Fatalf("acquireSourceLease(takeover) error = %v, want nil", err)
	}
	if got, want := outcome, sourceLeaseAcquired; got != want {
		t.Fatalf("acquireSourceLease(takeover) outcome = %q, want %q", got, want)
	}
	if takeover == nil {
		t.Fatal("acquireSourceLease(takeover) snapshot = nil, want non-nil")
	}
	if takeover.LeaseToken == "" || takeover.LeaseToken == first.LeaseToken {
		t.Fatalf("takeover LeaseToken = %q, want non-empty token distinct from %q", takeover.LeaseToken, first.LeaseToken)
	}

	renewed, err := renewSourceLease(ctx, first, pagesSourceSyncLeaseDuration)
	if err != nil {
		t.Fatalf("renewSourceLease(expired owner) error = %v, want nil", err)
	}
	if renewed {
		t.Error("renewSourceLease(expired owner) = true, want false")
	}
	if err := failSourceLease(ctx, first, "stale worker must not win"); err != nil {
		t.Fatalf("failSourceLease(expired owner) error = %v, want nil", err)
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&runtime).Error; err != nil {
		t.Fatalf("load runtime after takeover error = %v, want nil", err)
	}
	if got, want := runtime.LeaseToken, takeover.LeaseToken; got != want {
		t.Errorf("runtime LeaseToken after stale terminal write = %q, want %q", got, want)
	}
	if got, want := runtime.SyncStatus, pagesSourceStatusSyncing; got != want {
		t.Errorf("runtime SyncStatus after stale terminal write = %q, want %q", got, want)
	}

	renewed, err = renewSourceLease(ctx, takeover, pagesSourceSyncLeaseDuration)
	if err != nil {
		t.Fatalf("renewSourceLease(current owner) error = %v, want nil", err)
	}
	if !renewed {
		t.Error("renewSourceLease(current owner) = false, want true")
	}
	if err := failSourceLease(ctx, takeover, errPagesSourceSyncFailed); err != nil {
		t.Fatalf("failSourceLease(current owner) error = %v, want nil", err)
	}
	var failedRuntime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&failedRuntime).Error; err != nil {
		t.Fatalf("load failed runtime error = %v, want nil", err)
	}
	if got, want := failedRuntime.SyncStatus, pagesSourceStatusFailed; got != want {
		t.Errorf("failed runtime SyncStatus = %q, want %q", got, want)
	}
	if failedRuntime.LeaseToken != "" || failedRuntime.LeaseExpiresAt != nil {
		t.Errorf("failed runtime lease = (%q, %v), want cleared", failedRuntime.LeaseToken, failedRuntime.LeaseExpiresAt)
	}
}

func TestSourceConfigAndProjectContentChangesFenceLease(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "lease-fence")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip?token=first",
		false,
	)

	configSnapshot, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil || outcome != sourceLeaseAcquired {
		t.Fatalf("acquireSourceLease(config fence) = (%+v, %q, %v), want acquired", configSnapshot, outcome, err)
	}
	if _, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeRemoteURL,
		RemoteURL:     "https://example.com/site.zip?token=second",
		AllowInsecure: false,
	}); err != nil {
		t.Fatalf("UpdateSource(config fence) error = %v, want nil", err)
	}
	renewed, err := renewSourceLease(ctx, configSnapshot, pagesSourceSyncLeaseDuration)
	if err != nil {
		t.Fatalf("renewSourceLease(after source update) error = %v, want nil", err)
	}
	if renewed {
		t.Error("renewSourceLease(after source update) = true, want false")
	}
	var updatedSource model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ?", source.ID).First(&updatedSource).Error; err != nil {
		t.Fatalf("load updated source error = %v, want nil", err)
	}
	if got, want := updatedSource.ConfigVersion, source.ConfigVersion+1; got != want {
		t.Errorf("updated source ConfigVersion = %d, want %d", got, want)
	}
	if snapshot, staleOutcome, staleErr := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync); staleErr != nil || staleOutcome != sourceLeaseStale || snapshot != nil {
		t.Errorf("acquireSourceLease(old config) = (%+v, %q, %v), want (nil, %q, nil)", snapshot, staleOutcome, staleErr, sourceLeaseStale)
	}

	contentSnapshot, outcome, err := acquireSourceLease(ctx, source.ID, updatedSource.ConfigVersion, sourceActionSync)
	if err != nil || outcome != sourceLeaseAcquired {
		t.Fatalf("acquireSourceLease(content fence) = (%+v, %q, %v), want acquired", contentSnapshot, outcome, err)
	}
	if _, err := UpdateProject(ctx, project.ID, Input{
		Name:      project.Name,
		Slug:      project.Slug,
		Enabled:   true,
		RootDir:   "dist",
		EntryFile: "index.html",
	}); err != nil {
		t.Fatalf("UpdateProject(content fence) error = %v, want nil", err)
	}
	renewed, err = renewSourceLease(ctx, contentSnapshot, pagesSourceSyncLeaseDuration)
	if err != nil {
		t.Fatalf("renewSourceLease(after content update) error = %v, want nil", err)
	}
	if renewed {
		t.Error("renewSourceLease(after content update) = true, want false")
	}
	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", project.ID, err)
	}
	if got, want := storedProject.ContentConfigVersion, project.ContentConfigVersion+1; got != want {
		t.Errorf("ContentConfigVersion after RootDir update = %d, want %d", got, want)
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&runtime).Error; err != nil {
		t.Fatalf("load fenced runtime error = %v, want nil", err)
	}
	if runtime.LeaseToken != "" || runtime.LeaseExpiresAt != nil {
		t.Errorf("content-fenced runtime lease = (%q, %v), want cleared", runtime.LeaseToken, runtime.LeaseExpiresAt)
	}
}

func TestSourceRuntimeUsesOnlySixDocumentedStates(t *testing.T) {
	states := []string{
		pagesSourceStatusIdle,
		pagesSourceStatusChecking,
		pagesSourceStatusUpdateAvailable,
		pagesSourceStatusSyncing,
		pagesSourceStatusFailed,
		pagesSourceStatusAttention,
	}
	seen := make(map[string]struct{}, len(states))
	for _, state := range states {
		if strings.TrimSpace(state) == "" {
			t.Errorf("documented source state = %q, want non-empty", state)
		}
		if _, exists := seen[state]; exists {
			t.Errorf("documented source state %q is duplicated", state)
		}
		seen[state] = struct{}{}
	}
	if got, want := len(seen), 6; got != want {
		t.Errorf("unique source states = %d, want %d", got, want)
	}

	updateRuntime := &model.PagesProjectSourceRuntime{
		LastSeenRevision:    strings.Repeat("a", 64),
		LastAppliedRevision: strings.Repeat("b", 64),
		LastSeenDetail:      `{"release_id":"new"}`,
		LastAppliedDetail:   `{"release_id":"old"}`,
	}
	if got, want := normalizedSourceRuntimeStatus(updateRuntime), pagesSourceStatusUpdateAvailable; got != want {
		t.Errorf("normalizedSourceRuntimeStatus(update) = %q, want %q", got, want)
	}
	attentionRuntime := &model.PagesProjectSourceRuntime{
		LastSeenRevision:    strings.Repeat("a", 64),
		LastAppliedRevision: strings.Repeat("b", 64),
		LastSeenDetail:      `{"release_id":"same"}`,
		LastAppliedDetail:   `{"release_id":"same"}`,
	}
	if got, want := normalizedSourceRuntimeStatus(attentionRuntime), pagesSourceStatusAttention; got != want {
		t.Errorf("normalizedSourceRuntimeStatus(attention) = %q, want %q", got, want)
	}
	if got, want := normalizedSourceRuntimeStatus(&model.PagesProjectSourceRuntime{}), pagesSourceStatusIdle; got != want {
		t.Errorf("normalizedSourceRuntimeStatus(idle) = %q, want %q", got, want)
	}
}
