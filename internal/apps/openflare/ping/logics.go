// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package ping manages latency probe task scheduling, result ingestion, and node latency analytics.
package ping

import (
	"context"
	"errors"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	pingTypeICMP = "icmp"
	pingTypeTCP  = "tcp"
	pingTypeHTTP = "http"

	minPingIntervalSeconds     = 10
	defaultPingIntervalSeconds = 60
	maxPingIntervalSeconds     = 86400
	maxPingNameLength          = 255
	maxPingTargetLength        = 255

	defaultPingHours      = 1
	maxPingHours          = 720
	defaultPingMaxPoints  = 300
	minPingMaxPoints      = 50
	maxPingMaxPoints      = 2000
	maxPingRawQueryRows   = 50000
	percentScale          = 100
	ratioRoundingScale    = 100
	p50PercentileFraction = 0.50
	p99PercentileFraction = 0.99
)

// TaskInput is the payload for creating or updating a single ping task.
type TaskInput struct {
	Name      string   `json:"name"`
	Clients   []string `json:"clients"`
	DefaultOn bool     `json:"default_on"`
	Type      string   `json:"type"`
	Target    string   `json:"target"`
	Interval  int      `json:"interval"`
}

// TaskUpdateInput is one item in a batch ping task update request.
type TaskUpdateInput struct {
	ID        uint     `json:"id"`
	Name      string   `json:"name"`
	Clients   []string `json:"clients"`
	DefaultOn bool     `json:"default_on"`
	Type      string   `json:"type"`
	Target    string   `json:"target"`
	Interval  int      `json:"interval"`
}

// BatchUpdateTasksInput wraps a list of ping task updates.
type BatchUpdateTasksInput struct {
	Tasks []TaskUpdateInput `json:"tasks"`
}

// DeleteTasksInput deletes one or more ping tasks by ID.
type DeleteTasksInput struct {
	IDs []uint `json:"ids"`
}

// DeleteTasksResult reports how many ping tasks were deleted.
type DeleteTasksResult struct {
	Deleted int64 `json:"deleted"`
}

// NodePingRecordsQuery filters node latency records and statistics.
type NodePingRecordsQuery struct {
	TaskID    uint `form:"task_id" json:"task_id"`
	Hours     int  `form:"hours" json:"hours"`
	MaxPoints int  `form:"max_points" json:"max_points"`
}

// PingRecordView represents one timestamped latency sample (-1 indicates timeout/loss).
//
//nolint:revive // PingRecordView preserves Swagger schema and automation API type name
type PingRecordView struct {
	TaskID     uint      `json:"task_id"`
	RecordedAt time.Time `json:"recorded_at"`
	Value      int       `json:"value"`
}

// PingTaskStatsView summarizes unsampled latency statistics for one ping task on a node.
//
//nolint:revive // PingTaskStatsView preserves Swagger schema and automation API type name
type PingTaskStatsView struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Target      string  `json:"target"`
	Interval    int     `json:"interval"`
	Min         int     `json:"min"`
	Max         int     `json:"max"`
	Avg         float64 `json:"avg"`
	Latest      int     `json:"latest"`
	P50         int     `json:"p50"`
	P99         int     `json:"p99"`
	P99P50Ratio float64 `json:"p99_p50_ratio"`
	Loss        float64 `json:"loss"`
	Total       int     `json:"total"`
}

// NodePingRecordsView is the response payload for node latency charts and task statistics.
type NodePingRecordsView struct {
	NodeID  string              `json:"node_id"`
	Hours   int                 `json:"hours"`
	Tasks   []PingTaskStatsView `json:"tasks"`
	Records []PingRecordView    `json:"records"`
}

// ListTasks returns all configured ping tasks ordered by weight ASC, id ASC.
func ListTasks(ctx context.Context) ([]*model.OpenFlarePingTask, error) {
	tasks, err := repository.ListOpenFlarePingTasks(ctx)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task.Clients == nil {
			task.Clients = model.StringSliceJSON{}
		}
	}
	return tasks, nil
}

// GetTask returns one ping task by ID.
func GetTask(ctx context.Context, id uint) (*model.OpenFlarePingTask, error) {
	task, err := repository.GetOpenFlarePingTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Clients == nil {
		task.Clients = model.StringSliceJSON{}
	}
	return task, nil
}

// CreateTask validates and persists a new ping task, then reloads the scheduler.
func CreateTask(ctx context.Context, input TaskInput) (*model.OpenFlarePingTask, error) {
	if input.Interval == 0 {
		input.Interval = defaultPingIntervalSeconds
	}
	normalized, err := normalizeTaskInput(input)
	if err != nil {
		return nil, err
	}
	task := &model.OpenFlarePingTask{
		Name:      normalized.Name,
		Clients:   model.StringSliceJSON(normalized.Clients),
		DefaultOn: normalized.DefaultOn,
		Type:      normalized.Type,
		Target:    normalized.Target,
		Interval:  normalized.Interval,
	}
	if err := repository.CreateOpenFlarePingTask(ctx, task); err != nil {
		return nil, err
	}
	ofws.NotifyPingScheduleChanged(ctx)
	return task, nil
}

// UpdateTask updates a single ping task by ID and reloads the scheduler.
func UpdateTask(ctx context.Context, id uint, input TaskInput) (*model.OpenFlarePingTask, error) {
	if id == 0 {
		return nil, errors.New(errPingTaskNotFound)
	}
	normalized, err := normalizeTaskInput(input)
	if err != nil {
		return nil, err
	}
	task := &model.OpenFlarePingTask{
		ID:        id,
		Name:      normalized.Name,
		Clients:   model.StringSliceJSON(normalized.Clients),
		DefaultOn: normalized.DefaultOn,
		Type:      normalized.Type,
		Target:    normalized.Target,
		Interval:  normalized.Interval,
	}
	if err := repository.UpdateOpenFlarePingTasks(ctx, []*model.OpenFlarePingTask{task}); err != nil {
		return nil, err
	}
	ofws.NotifyPingScheduleChanged(ctx)
	return GetTask(ctx, id)
}

// UpdateTasks batch updates multiple ping tasks atomically and reloads the scheduler.
func UpdateTasks(ctx context.Context, inputs []TaskUpdateInput) ([]*model.OpenFlarePingTask, error) {
	if len(inputs) == 0 {
		return nil, errors.New(errPingTaskBatchRequired)
	}
	models := make([]*model.OpenFlarePingTask, 0, len(inputs))
	for _, item := range inputs {
		if item.ID == 0 {
			return nil, errors.New(errPingTaskNotFound)
		}
		normalized, err := normalizeTaskInput(TaskInput{
			Name:      item.Name,
			Clients:   item.Clients,
			DefaultOn: item.DefaultOn,
			Type:      item.Type,
			Target:    item.Target,
			Interval:  item.Interval,
		})
		if err != nil {
			return nil, err
		}
		models = append(models, &model.OpenFlarePingTask{
			ID:        item.ID,
			Name:      normalized.Name,
			Clients:   model.StringSliceJSON(normalized.Clients),
			DefaultOn: normalized.DefaultOn,
			Type:      normalized.Type,
			Target:    normalized.Target,
			Interval:  normalized.Interval,
		})
	}
	if err := repository.UpdateOpenFlarePingTasks(ctx, models); err != nil {
		return nil, err
	}
	ofws.NotifyPingScheduleChanged(ctx)
	return ListTasks(ctx)
}

// DeleteTasks deletes ping tasks and their associated latency history, then reloads the scheduler.
func DeleteTasks(ctx context.Context, ids []uint) (int64, error) {
	uniqueIDs := dedupeUintIDs(ids)
	if len(uniqueIDs) == 0 {
		return 0, errors.New(errPingTaskIDsRequired)
	}
	if _, err := repository.DeleteOpenFlarePingRecordsByTaskIDs(ctx, uniqueIDs); err != nil {
		logger.WarnF(ctx, "[Ping] failed to purge ping records for deleted tasks %v: %v", uniqueIDs, err)
	}
	deleted, err := repository.DeleteOpenFlarePingTasks(ctx, uniqueIDs)
	if err != nil {
		return 0, err
	}
	ofws.NotifyPingScheduleChanged(ctx)
	return deleted, nil
}

// ReorderTasks updates display weights for ping tasks and reloads the scheduler.
func ReorderTasks(ctx context.Context, weights map[uint]int) error {
	if len(weights) == 0 {
		return errors.New(errPingTaskWeightsRequired)
	}
	for id := range weights {
		if id == 0 {
			return errors.New(errPingTaskWeightIDInvalid)
		}
	}
	if err := repository.ReorderOpenFlarePingTasks(ctx, weights); err != nil {
		return err
	}
	ofws.NotifyPingScheduleChanged(ctx)
	return nil
}

// RecordResult persists a ping measurement reported over WebSocket (logging errors instead of returning).
func RecordResult(ctx context.Context, nodeID string, payload protocol.PingResultPayload) {
	if err := RecordResultWithError(ctx, nodeID, payload); err != nil {
		logger.WarnF(ctx, "[Ping] failed to persist ping result for node=%s task=%d: %v", nodeID, payload.TaskID, err)
	}
}

// RecordResultWithError validates and persists a ping measurement into logstore.
func RecordResultWithError(ctx context.Context, nodeID string, payload protocol.PingResultPayload) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || payload.TaskID == 0 {
		return errors.New(errPingResultInvalid)
	}
	recordedAt := parseRFC3339OrNow(payload.FinishedAt)
	value := payload.Value
	if value < -1 {
		value = -1
	}
	return repository.InsertOpenFlarePingRecord(ctx, &model.OpenFlarePingRecord{
		NodeID:     nodeID,
		TaskID:     uint(payload.TaskID),
		RecordedAt: recordedAt,
		Value:      value,
	})
}

// AttachNewNode binds a newly created/registered node to all default_on ping tasks.
func AttachNewNode(ctx context.Context, nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return
	}
	if err := repository.AttachNodeToDefaultOnPingTasks(ctx, nodeID); err != nil {
		logger.WarnF(ctx, "[Ping] failed to attach node %s to default_on ping tasks: %v", nodeID, err)
		return
	}
	ofws.NotifyPingScheduleChanged(ctx)
}

// DetachDeletedNode removes a deleted node from all ping tasks.
func DetachDeletedNode(ctx context.Context, nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return
	}
	if err := repository.RemoveNodeFromAllPingTasks(ctx, nodeID); err != nil {
		logger.WarnF(ctx, "[Ping] failed to detach deleted node %s from ping tasks: %v", nodeID, err)
		return
	}
	ofws.NotifyPingScheduleChanged(ctx)
}

// GetNodePingRecords returns bound ping tasks with unsampled statistics and downsampled series for a node.
func GetNodePingRecords(ctx context.Context, nodeRecordID uint, query NodePingRecordsQuery) (*NodePingRecordsView, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, nodeRecordID)
	if err != nil {
		return nil, err
	}
	hours := normalizePingHours(query.Hours)
	maxPoints := normalizePingMaxPoints(query.MaxPoints)
	now := time.Now().UTC()
	since := now.Add(-time.Duration(hours) * time.Hour)

	allTasks, err := repository.ListOpenFlarePingTasks(ctx)
	if err != nil {
		return nil, err
	}
	relevantTasks := make([]*model.OpenFlarePingTask, 0, len(allTasks))
	taskByID := make(map[uint]*model.OpenFlarePingTask, len(allTasks))
	for _, task := range allTasks {
		if query.TaskID > 0 {
			if task.ID == query.TaskID {
				relevantTasks = append(relevantTasks, task)
				taskByID[task.ID] = task
			}
			continue
		}
		if slices.Contains(task.Clients, node.NodeID) {
			relevantTasks = append(relevantTasks, task)
			taskByID[task.ID] = task
		}
	}

	rawRecords, err := repository.ListOpenFlarePingRecords(ctx, node.NodeID, query.TaskID, since, maxPingRawQueryRows)
	if err != nil {
		return nil, err
	}

	// Sort chronologically ascending so latest and downsampling behave deterministically.
	sort.Slice(rawRecords, func(i, j int) bool {
		if rawRecords[i].RecordedAt.Equal(rawRecords[j].RecordedAt) {
			return rawRecords[i].TaskID < rawRecords[j].TaskID
		}
		return rawRecords[i].RecordedAt.Before(rawRecords[j].RecordedAt)
	})

	byTask := make(map[uint][]*model.OpenFlarePingRecord, len(relevantTasks))
	for _, rec := range rawRecords {
		if rec == nil {
			continue
		}
		if _, ok := taskByID[rec.TaskID]; !ok {
			continue
		}
		byTask[rec.TaskID] = append(byTask[rec.TaskID], rec)
	}

	taskStats := make([]PingTaskStatsView, 0, len(relevantTasks))
	outRecords := make([]PingRecordView, 0, min(len(rawRecords), len(relevantTasks)*maxPoints))
	for _, task := range relevantTasks {
		recs := byTask[task.ID]
		taskStats = append(taskStats, computeTaskStats(task, recs))
		sampled := downsamplePingRecords(recs, since, now, maxPoints)
		outRecords = append(outRecords, sampled...)
	}

	sort.Slice(outRecords, func(i, j int) bool {
		if outRecords[i].RecordedAt.Equal(outRecords[j].RecordedAt) {
			return outRecords[i].TaskID < outRecords[j].TaskID
		}
		return outRecords[i].RecordedAt.Before(outRecords[j].RecordedAt)
	})

	return &NodePingRecordsView{
		NodeID:  node.NodeID,
		Hours:   hours,
		Tasks:   taskStats,
		Records: outRecords,
	}, nil
}

func computeTaskStats(task *model.OpenFlarePingTask, records []*model.OpenFlarePingRecord) PingTaskStatsView {
	stats := PingTaskStatsView{
		ID:       task.ID,
		Name:     task.Name,
		Type:     task.Type,
		Target:   task.Target,
		Interval: task.Interval,
		Latest:   -1,
		Total:    len(records),
	}
	if len(records) == 0 {
		return stats
	}

	stats.Latest = records[len(records)-1].Value
	valid := make([]int, 0, len(records))
	var sum int64
	lost := 0
	for _, rec := range records {
		if rec.Value < 0 {
			lost++
			continue
		}
		valid = append(valid, rec.Value)
		sum += int64(rec.Value)
	}

	stats.Loss = roundTwo((float64(lost) / float64(len(records))) * percentScale)
	if len(valid) == 0 {
		return stats
	}

	sort.Ints(valid)
	stats.Min = valid[0]
	stats.Max = valid[len(valid)-1]
	stats.Avg = roundTwo(float64(sum) / float64(len(valid)))
	stats.P50 = percentileInt(valid, p50PercentileFraction)
	stats.P99 = percentileInt(valid, p99PercentileFraction)
	if stats.P50 > 0 {
		stats.P99P50Ratio = roundTwo(float64(stats.P99) / float64(stats.P50))
	}
	return stats
}

func downsamplePingRecords(records []*model.OpenFlarePingRecord, since, now time.Time, maxPoints int) []PingRecordView {
	if len(records) == 0 {
		return nil
	}
	if len(records) <= maxPoints || maxPoints <= 0 {
		out := make([]PingRecordView, len(records))
		for i, rec := range records {
			out[i] = PingRecordView{
				TaskID:     rec.TaskID,
				RecordedAt: rec.RecordedAt.UTC(),
				Value:      rec.Value,
			}
		}
		return out
	}

	span := now.Sub(since)
	if span <= 0 {
		span = time.Hour
	}
	bucketDur := span / time.Duration(maxPoints)
	if bucketDur <= 0 {
		bucketDur = time.Second
	}

	type bucket struct {
		taskID     uint
		validSum   int64
		validCount int
		lossCount  int
		lastValid  time.Time
		lastLoss   time.Time
	}

	buckets := make(map[int]*bucket, maxPoints)
	keys := make([]int, 0, maxPoints)
	for _, rec := range records {
		idx := int(rec.RecordedAt.Sub(since) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= maxPoints {
			idx = maxPoints - 1
		}
		b, ok := buckets[idx]
		if !ok {
			b = &bucket{taskID: rec.TaskID}
			buckets[idx] = b
			keys = append(keys, idx)
		}
		if rec.Value < 0 {
			b.lossCount++
			b.lastLoss = rec.RecordedAt.UTC()
		} else {
			b.validSum += int64(rec.Value)
			b.validCount++
			b.lastValid = rec.RecordedAt.UTC()
		}
	}
	sort.Ints(keys)

	out := make([]PingRecordView, 0, len(keys))
	for _, idx := range keys {
		b := buckets[idx]
		if b.validCount > 0 {
			out = append(out, PingRecordView{
				TaskID:     b.taskID,
				RecordedAt: b.lastValid,
				Value:      int(math.Round(float64(b.validSum) / float64(b.validCount))),
			})
		}
		if b.lossCount > 0 {
			// Preserve loss markers when a bucket experienced packet loss.
			if b.validCount == 0 || b.lossCount*2 >= b.validCount+b.lossCount {
				out = append(out, PingRecordView{
					TaskID:     b.taskID,
					RecordedAt: b.lastLoss,
					Value:      -1,
				})
			}
		}
	}
	return out
}

func percentileInt(sorted []int, fraction float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*fraction)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func roundTwo(v float64) float64 {
	return math.Round(v*ratioRoundingScale) / ratioRoundingScale
}

func normalizeTaskInput(input TaskInput) (TaskInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return TaskInput{}, errors.New(errPingTaskNameRequired)
	}
	if len(name) > maxPingNameLength {
		return TaskInput{}, errors.New(errPingTaskNameTooLong)
	}
	pingType := strings.ToLower(strings.TrimSpace(input.Type))
	switch pingType {
	case pingTypeICMP, pingTypeTCP, pingTypeHTTP:
	default:
		return TaskInput{}, errors.New(errPingTaskTypeInvalid)
	}
	target := strings.TrimSpace(input.Target)
	if target == "" {
		return TaskInput{}, errors.New(errPingTaskTargetRequired)
	}
	if len(target) > maxPingTargetLength {
		return TaskInput{}, errors.New(errPingTaskTargetTooLong)
	}
	if input.Interval < minPingIntervalSeconds || input.Interval > maxPingIntervalSeconds {
		return TaskInput{}, errors.New(errPingTaskIntervalInvalid)
	}
	clients := dedupeStrings(input.Clients)
	return TaskInput{
		Name:      name,
		Clients:   clients,
		DefaultOn: input.DefaultOn,
		Type:      pingType,
		Target:    target,
		Interval:  input.Interval,
	}, nil
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
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

func dedupeUintIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizePingHours(hours int) int {
	if hours <= 0 {
		return defaultPingHours
	}
	if hours > maxPingHours {
		return maxPingHours
	}
	return hours
}

func normalizePingMaxPoints(maxPoints int) int {
	if maxPoints <= 0 {
		return defaultPingMaxPoints
	}
	if maxPoints < minPingMaxPoints {
		return minPingMaxPoints
	}
	if maxPoints > maxPingMaxPoints {
		return maxPingMaxPoints
	}
	return maxPoints
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
