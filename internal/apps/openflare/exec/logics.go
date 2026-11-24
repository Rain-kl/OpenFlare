// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package exec manages remote shell command execution tasks dispatched to edge nodes.
package exec

import (
	"context"
	"errors"
	"strings"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/google/uuid"
)

const (
	defaultExecTimeoutSeconds = 60
	maxExecTimeoutSeconds     = 600
	maxExecCommandLength      = 32768
	maxExecOutputLength       = 256 * 1024
	defaultExecListLimit      = 50
	maxExecListLimit          = 200
	nodeOnlineThreshold       = 60 * time.Second
	execRetentionDays         = 30
)

// RunInput specifies a remote command execution request.
type RunInput struct {
	Command        string   `json:"command"`
	Clients        []string `json:"clients"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// TaskResultView represents one node's execution result enriched with the node display name.
type TaskResultView struct {
	ID         uint       `json:"id,omitempty"`
	TaskID     string     `json:"task_id"`
	Client     string     `json:"client"`
	NodeName   string     `json:"node_name"`
	Result     string     `json:"result"`
	ExitCode   *int       `json:"exit_code"`
	FinishedAt *time.Time `json:"finished_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// TaskView represents a remote execution task with per-node results.
type TaskView struct {
	TaskID    string           `json:"task_id"`
	Command   string           `json:"command"`
	Clients   []string         `json:"clients"`
	CreatedBy uint             `json:"created_by"`
	CreatedAt time.Time        `json:"created_at"`
	Results   []TaskResultView `json:"results"`
}

// RunResponse reports task creation and per-node dispatch status.
type RunResponse struct {
	TaskID         string           `json:"task_id"`
	Command        string           `json:"command"`
	Clients        []string         `json:"clients"`
	OnlineClients  []string         `json:"online_clients"`
	OfflineClients []string         `json:"offline_clients"`
	CreatedAt      time.Time        `json:"created_at"`
	Results        []TaskResultView `json:"results"`
}

// RunCommand persists a remote execution task and dispatches it to target edge nodes.
func RunCommand(ctx context.Context, userID uint, input RunInput) (*RunResponse, error) {
	if strings.TrimSpace(input.Command) == "" {
		return nil, errors.New(errExecCommandRequired)
	}
	if len(input.Command) > maxExecCommandLength {
		return nil, errors.New(errExecCommandTooLong)
	}
	requestedClients := dedupeStrings(input.Clients)
	if len(requestedClients) == 0 {
		return nil, errors.New(errExecClientsRequired)
	}

	timeoutSeconds := normalizeExecTimeout(input.TimeoutSeconds)
	nodes, err := repository.ListOpenFlareNodesByNodeIDs(ctx, requestedClients)
	if err != nil {
		return nil, err
	}
	nodeByID := make(map[string]model.OpenFlareNode, len(nodes))
	for _, n := range nodes {
		nodeByID[n.NodeID] = n
	}

	validClients := make([]string, 0, len(requestedClients))
	for _, clientID := range requestedClients {
		if _, ok := nodeByID[clientID]; ok {
			validClients = append(validClients, clientID)
		}
	}
	if len(validClients) == 0 {
		return nil, errors.New(errExecNoValidClients)
	}

	now := time.Now().UTC()
	taskID := "exec-" + uuid.NewString()
	initialResults := make([]model.OpenFlareExecTaskResult, 0, len(validClients))
	dispatchTargets := make([]string, 0, len(validClients))
	offlineClients := make([]string, 0)

	for _, clientID := range validClients {
		n := nodeByID[clientID]
		switch {
		case n.IsQuarantined():
			exitCode := -1
			finished := now
			initialResults = append(initialResults, model.OpenFlareExecTaskResult{
				TaskID:     taskID,
				Client:     clientID,
				Result:     errExecNodeQuarantined,
				ExitCode:   &exitCode,
				FinishedAt: &finished,
				CreatedAt:  now,
			})
			offlineClients = append(offlineClients, clientID)
		case !isNodeReachable(&n, now):
			exitCode := -1
			finished := now
			initialResults = append(initialResults, model.OpenFlareExecTaskResult{
				TaskID:     taskID,
				Client:     clientID,
				Result:     errExecNodeOffline,
				ExitCode:   &exitCode,
				FinishedAt: &finished,
				CreatedAt:  now,
			})
			offlineClients = append(offlineClients, clientID)
		default:
			initialResults = append(initialResults, model.OpenFlareExecTaskResult{
				TaskID:    taskID,
				Client:    clientID,
				Result:    "",
				CreatedAt: now,
			})
			dispatchTargets = append(dispatchTargets, clientID)
		}
	}

	task := &model.OpenFlareExecTask{
		TaskID:    taskID,
		Clients:   model.StringSliceJSON(validClients),
		Command:   input.Command,
		CreatedBy: userID,
		CreatedAt: now,
	}
	// Persist before dispatching so fast command completions never race database creation.
	if err := repository.CreateOpenFlareExecTask(ctx, task, initialResults); err != nil {
		return nil, err
	}

	onlineClients := make([]string, 0, len(dispatchTargets))
	dispatch := protocol.RemoteExecDispatch{
		TaskID:         taskID,
		Command:        input.Command,
		TimeoutSeconds: timeoutSeconds,
	}
	for _, clientID := range dispatchTargets {
		if ofws.DispatchRemoteExec(ctx, clientID, dispatch) {
			onlineClients = append(onlineClients, clientID)
			continue
		}
		offlineClients = append(offlineClients, clientID)
		finished := time.Now().UTC()
		if saveErr := repository.SaveOpenFlareExecTaskResult(ctx, taskID, clientID, errExecNodeDisconnected, -1, finished); saveErr != nil {
			logger.WarnF(ctx, "[Exec] failed to mark disconnected client result task=%s node=%s: %v", taskID, clientID, saveErr)
		}
		for i := range initialResults {
			if initialResults[i].Client == clientID {
				exitCode := -1
				initialResults[i].Result = errExecNodeDisconnected
				initialResults[i].ExitCode = &exitCode
				initialResults[i].FinishedAt = &finished
				break
			}
		}
	}

	resultViews := make([]TaskResultView, 0, len(initialResults))
	for _, r := range initialResults {
		resultViews = append(resultViews, TaskResultView{
			ID:         r.ID,
			TaskID:     r.TaskID,
			Client:     r.Client,
			NodeName:   nodeByID[r.Client].Name,
			Result:     r.Result,
			ExitCode:   r.ExitCode,
			FinishedAt: r.FinishedAt,
			CreatedAt:  r.CreatedAt,
		})
	}

	return &RunResponse{
		TaskID:         taskID,
		Command:        task.Command,
		Clients:        validClients,
		OnlineClients:  onlineClients,
		OfflineClients: offlineClients,
		CreatedAt:      now,
		Results:        resultViews,
	}, nil
}

// RecordResult persists a node's reported remote execution output and exit code.
func RecordResult(ctx context.Context, nodeID string, payload protocol.RemoteExecResultPayload) error {
	nodeID = strings.TrimSpace(nodeID)
	taskID := strings.TrimSpace(payload.TaskID)
	if nodeID == "" || taskID == "" {
		return errors.New(errExecResultInvalid)
	}
	output := payload.Result
	if len(output) > maxExecOutputLength {
		output = output[:maxExecOutputLength] + "\n...[output truncated]"
	}
	finishedAt := parseRFC3339OrNow(payload.FinishedAt)
	return repository.SaveOpenFlareExecTaskResult(ctx, taskID, nodeID, output, payload.ExitCode, finishedAt)
}

// ListTasks returns recent remote execution tasks enriched with node names.
func ListTasks(ctx context.Context, limit int) ([]*TaskView, error) {
	if limit <= 0 {
		limit = defaultExecListLimit
	}
	if limit > maxExecListLimit {
		limit = maxExecListLimit
	}
	tasks, err := repository.ListOpenFlareExecTasks(ctx, limit)
	if err != nil {
		return nil, err
	}
	nodeIDSet := make(map[string]struct{})
	for _, task := range tasks {
		for _, c := range task.Clients {
			nodeIDSet[c] = struct{}{}
		}
		for _, r := range task.Results {
			nodeIDSet[r.Client] = struct{}{}
		}
	}
	nameByID := loadNodeNames(ctx, nodeIDSet)
	views := make([]*TaskView, 0, len(tasks))
	for _, task := range tasks {
		views = append(views, buildTaskView(task, nameByID))
	}
	return views, nil
}

// GetTask loads one remote execution task with its per-node results and node names.
func GetTask(ctx context.Context, taskID string) (*TaskView, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New(errExecTaskIDRequired)
	}
	task, err := repository.GetOpenFlareExecTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	nodeIDSet := make(map[string]struct{}, len(task.Clients)+len(task.Results))
	for _, c := range task.Clients {
		nodeIDSet[c] = struct{}{}
	}
	for _, r := range task.Results {
		nodeIDSet[r.Client] = struct{}{}
	}
	nameByID := loadNodeNames(ctx, nodeIDSet)
	return buildTaskView(task, nameByID), nil
}

// DeleteTask deletes one remote execution task and its results.
func DeleteTask(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New(errExecTaskIDRequired)
	}
	return repository.DeleteOpenFlareExecTask(ctx, taskID)
}

// CleanupExpired purges remote execution tasks older than execRetentionDays.
func CleanupExpired(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -execRetentionDays)
	return repository.DeleteOpenFlareExecTasksBefore(ctx, cutoff)
}

func buildTaskView(task *model.OpenFlareExecTask, nameByID map[string]string) *TaskView {
	clients := []string(task.Clients)
	if clients == nil {
		clients = []string{}
	}
	results := make([]TaskResultView, 0, len(task.Results))
	for _, r := range task.Results {
		results = append(results, TaskResultView{
			ID:         r.ID,
			TaskID:     r.TaskID,
			Client:     r.Client,
			NodeName:   nameByID[r.Client],
			Result:     r.Result,
			ExitCode:   r.ExitCode,
			FinishedAt: r.FinishedAt,
			CreatedAt:  r.CreatedAt,
		})
	}
	return &TaskView{
		TaskID:    task.TaskID,
		Command:   task.Command,
		Clients:   clients,
		CreatedBy: task.CreatedBy,
		CreatedAt: task.CreatedAt,
		Results:   results,
	}
}

func loadNodeNames(ctx context.Context, idSet map[string]struct{}) map[string]string {
	out := make(map[string]string, len(idSet))
	if len(idSet) == 0 {
		return out
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	nodes, err := repository.ListOpenFlareNodesByNodeIDs(ctx, ids)
	if err != nil {
		logger.WarnF(ctx, "[Exec] failed to resolve node names for remote exec tasks: %v", err)
		return out
	}
	for _, n := range nodes {
		out[n.NodeID] = n.Name
	}
	return out
}

func isNodeReachable(n *model.OpenFlareNode, now time.Time) bool {
	if ofws.IsAgentConnected(n.NodeID) {
		return true
	}
	if n.LastSeenAt == nil || n.LastSeenAt.IsZero() {
		return false
	}
	return now.Sub(*n.LastSeenAt) <= nodeOnlineThreshold
}

func normalizeExecTimeout(timeout int) int {
	if timeout <= 0 {
		return defaultExecTimeoutSeconds
	}
	if timeout > maxExecTimeoutSeconds {
		return maxExecTimeoutSeconds
	}
	return timeout
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func parseRFC3339OrNow(raw string) time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now().UTC()
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC()
	}
	return time.Now().UTC()
}
