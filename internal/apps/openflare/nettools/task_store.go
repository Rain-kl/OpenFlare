// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nettools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netquality"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	taskTTL             = 15 * time.Minute
	taskPendingTimeout  = 45 * time.Second
	maxLocalCachedTasks = 512
	taskRedisKeyPrefix  = "nettools:task:"

	// TaskStatusRunning indicates at least one node probe is still active.
	TaskStatusRunning = "running"
	// TaskStatusCompleted indicates every targeted node finished or was marked offline/timed out.
	TaskStatusCompleted = "completed"
	// TaskStatusTimeout indicates the multi-node probe timed out before all nodes responded.
	TaskStatusTimeout = "timeout"

	// NodeResultStatusPending indicates an edge node has not yet returned its probe result.
	NodeResultStatusPending = "pending"
	// NodeResultStatusCompleted indicates an edge node completed the probe successfully.
	NodeResultStatusCompleted = "completed"
	// NodeResultStatusFailed indicates an edge node reported an execution failure.
	NodeResultStatusFailed = "failed"
	// NodeResultStatusOffline indicates the edge node had no live WebSocket connection at dispatch time.
	NodeResultStatusOffline = "offline"
	// NodeResultStatusTimeout indicates the edge node did not respond within the waiting window.
	NodeResultStatusTimeout = "timeout"
)

// EnrichedTracerouteResult extends TracerouteResult with per-hop ASN and GeoIP enrichment.
type EnrichedTracerouteResult struct {
	Target     string                             `json:"target"`
	ResolvedIP string                             `json:"resolved_ip,omitempty"`
	Protocol   string                             `json:"protocol"`
	HopCount   uint8                              `json:"hop_count"`
	Reached    bool                               `json:"reached"`
	TotalRTTMs float64                            `json:"total_rtt_ms"`
	Hops       []netquality.EnrichedTracerouteHop `json:"hops"`
	Error      string                             `json:"error,omitempty"`
}

// ProbeNodeResult stores the outcome from a single edge node within an active multi-node probe task.
type ProbeNodeResult struct {
	NodeID     string                    `json:"node_id"`
	NodeName   string                    `json:"node_name"`
	NodeGeo    string                    `json:"node_geo"`
	NodeASN    uint32                    `json:"node_asn"`
	Status     string                    `json:"status"`
	Ping       *protocol.PingBatchResult `json:"ping,omitempty"`
	Traceroute *EnrichedTracerouteResult `json:"traceroute,omitempty"`
	DNS        *protocol.DNSQueryResult  `json:"dns,omitempty"`
	Error      string                    `json:"error,omitempty"`
	FinishedAt string                    `json:"finished_at,omitempty"`
}

// ProbeTaskState tracks execution state and per-node results for one multi-node diagnostic run.
type ProbeTaskState struct {
	TaskID         string             `json:"task_id"`
	Kind           string             `json:"kind"`
	Target         string             `json:"target"`
	CreatedAt      time.Time          `json:"created_at"`
	ExpiresAt      time.Time          `json:"expires_at"`
	TotalNodes     int                `json:"total_nodes"`
	CompletedNodes int                `json:"completed_nodes"`
	Status         string             `json:"status"`
	Nodes          []*ProbeNodeResult `json:"nodes"`
}

type taskMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*ProbeTaskState
	order []string
}

var defaultTaskStore = &taskMemoryStore{
	tasks: make(map[string]*ProbeTaskState),
}

// ResetTaskStoreForTest clears the in-memory task coordinator cache for tests.
func ResetTaskStoreForTest() {
	defaultTaskStore.mu.Lock()
	defaultTaskStore.tasks = make(map[string]*ProbeTaskState)
	defaultTaskStore.order = nil
	defaultTaskStore.mu.Unlock()
}

func cloneTaskState(src *ProbeTaskState) *ProbeTaskState {
	if src == nil {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return src
	}
	var copyState ProbeTaskState
	if err := json.Unmarshal(raw, &copyState); err != nil {
		return src
	}
	return &copyState
}

func saveTaskState(ctx context.Context, state *ProbeTaskState) {
	if state == nil || state.TaskID == "" {
		return
	}
	recomputeTaskCounters(state, time.Now().UTC())
	snapshot := cloneTaskState(state)

	defaultTaskStore.mu.Lock()
	if _, exists := defaultTaskStore.tasks[snapshot.TaskID]; !exists {
		defaultTaskStore.order = append(defaultTaskStore.order, snapshot.TaskID)
		for len(defaultTaskStore.order) > maxLocalCachedTasks {
			oldest := defaultTaskStore.order[0]
			defaultTaskStore.order = defaultTaskStore.order[1:]
			delete(defaultTaskStore.tasks, oldest)
		}
	}
	defaultTaskStore.tasks[snapshot.TaskID] = snapshot
	defaultTaskStore.mu.Unlock()

	if db.Redis != nil {
		if encoded, err := json.Marshal(snapshot); err == nil {
			key := db.PrefixedKey(taskRedisKeyPrefix + snapshot.TaskID)
			if err := db.Redis.Set(ctx, key, string(encoded), taskTTL).Err(); err != nil {
				logger.WarnF(ctx, "[NetTools] persist task %s in Redis failed: %v", snapshot.TaskID, err)
			}
		}
	}
}

func loadTaskState(ctx context.Context, taskID string) (*ProbeTaskState, error) {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return nil, errors.New(errTaskNotFound)
	}

	if db.Redis != nil {
		key := db.PrefixedKey(taskRedisKeyPrefix + id)
		raw, err := db.Redis.Get(ctx, key).Result()
		if err == nil && raw != "" {
			var parsed ProbeTaskState
			if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
				recomputeTaskCounters(&parsed, time.Now().UTC())
				return &parsed, nil
			}
		}
	}

	defaultTaskStore.mu.RLock()
	cached := defaultTaskStore.tasks[id]
	defaultTaskStore.mu.RUnlock()
	if cached == nil {
		return nil, errors.New(errTaskNotFound)
	}
	now := time.Now().UTC()
	if !cached.ExpiresAt.IsZero() && now.After(cached.ExpiresAt) {
		defaultTaskStore.mu.Lock()
		delete(defaultTaskStore.tasks, id)
		defaultTaskStore.mu.Unlock()
		return nil, errors.New(errTaskNotFound)
	}
	copyState := cloneTaskState(cached)
	recomputeTaskCounters(copyState, now)
	return copyState, nil
}

func recomputeTaskCounters(state *ProbeTaskState, now time.Time) {
	if state == nil {
		return
	}
	state.TotalNodes = len(state.Nodes)
	completed := 0
	finishedOrOffline := 0
	pendingTimedOut := !state.CreatedAt.IsZero() && now.Sub(state.CreatedAt) > taskPendingTimeout
	for _, n := range state.Nodes {
		if n == nil {
			continue
		}
		if n.Status == NodeResultStatusPending && pendingTimedOut {
			n.Status = NodeResultStatusTimeout
			if n.Error == "" {
				n.Error = "probe timed out awaiting node response"
			}
		}
		switch n.Status {
		case NodeResultStatusCompleted, NodeResultStatusFailed:
			completed++
			finishedOrOffline++
		case NodeResultStatusOffline, NodeResultStatusTimeout:
			finishedOrOffline++
		}
	}
	state.CompletedNodes = completed
	if state.TotalNodes > 0 && finishedOrOffline >= state.TotalNodes {
		state.Status = TaskStatusCompleted
	} else {
		state.Status = TaskStatusRunning
	}
}

// HandleNetProbeResult is registered in bootstrap as the agent WS & HTTP callback for net_probe_result frames.
// It always feeds passive/active quality matrix intelligence and updates any active interactive diagnostic task.
func HandleNetProbeResult(ctx context.Context, nodeID string, payload protocol.NetProbeResultPayload) {
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorF(ctx, "[NetTools] HandleNetProbeResult panic recovered: %v", r)
		}
	}()

	// Feed quality matrix + AS topology graph for both auto-scheduled and manual probes.
	netquality.IngestProbeResult(ctx, nodeID, payload)

	probeID := strings.TrimSpace(payload.ProbeID)
	trimmedNode := strings.TrimSpace(nodeID)
	if probeID == "" || trimmedNode == "" {
		return
	}

	defaultTaskStore.mu.Lock()
	state, err := loadTaskStateForUpdateLocked(ctx, probeID)
	if err != nil || state == nil {
		defaultTaskStore.mu.Unlock()
		return
	}

	var matched *ProbeNodeResult
	for _, n := range state.Nodes {
		if n != nil && n.NodeID == trimmedNode {
			matched = n
			break
		}
	}
	if matched == nil {
		matched = &ProbeNodeResult{NodeID: trimmedNode}
		state.Nodes = append(state.Nodes, matched)
	}

	matched.Ping = payload.Ping
	matched.DNS = payload.DNS
	matched.Error = strings.TrimSpace(payload.Error)
	matched.FinishedAt = strings.TrimSpace(payload.FinishedAt)
	if matched.FinishedAt == "" {
		matched.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if payload.Traceroute != nil {
		matched.Traceroute = &EnrichedTracerouteResult{
			Target:     payload.Traceroute.Target,
			ResolvedIP: payload.Traceroute.ResolvedIP,
			Protocol:   payload.Traceroute.Protocol,
			HopCount:   payload.Traceroute.HopCount,
			Reached:    payload.Traceroute.Reached,
			TotalRTTMs: payload.Traceroute.TotalRTTMs,
			Hops:       netquality.EnrichTracerouteHops(payload.Traceroute.Hops),
			Error:      payload.Traceroute.Error,
		}
	}
	if matched.Error != "" {
		matched.Status = NodeResultStatusFailed
	} else {
		matched.Status = NodeResultStatusCompleted
	}
	recomputeTaskCounters(state, time.Now().UTC())
	snapshot := cloneTaskState(state)
	defaultTaskStore.tasks[snapshot.TaskID] = snapshot
	defaultTaskStore.mu.Unlock()

	if db.Redis != nil {
		if encoded, mErr := json.Marshal(snapshot); mErr == nil {
			key := db.PrefixedKey(taskRedisKeyPrefix + snapshot.TaskID)
			if sErr := db.Redis.Set(ctx, key, string(encoded), taskTTL).Err(); sErr != nil {
				logger.WarnF(ctx, "[NetTools] update task %s in Redis failed: %v", snapshot.TaskID, sErr)
			}
		}
	}
}

func loadTaskStateForUpdateLocked(ctx context.Context, probeID string) (*ProbeTaskState, error) {
	if cached := defaultTaskStore.tasks[probeID]; cached != nil {
		return cloneTaskState(cached), nil
	}
	if db.Redis != nil {
		key := db.PrefixedKey(taskRedisKeyPrefix + probeID)
		raw, err := db.Redis.Get(ctx, key).Result()
		if err == nil && raw != "" {
			var parsed ProbeTaskState
			if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
				return &parsed, nil
			}
		}
	}
	return nil, errors.New(errTaskNotFound)
}
