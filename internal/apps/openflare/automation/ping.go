// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/ping"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// PingTaskSummary describes a configured latency probe task.
type PingTaskSummary struct {
	TaskID    string    `json:"task_id"`
	Weight    int       `json:"weight"`
	Name      string    `json:"name"`
	Clients   []string  `json:"clients"`
	DefaultOn bool      `json:"default_on"`
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	Interval  int       `json:"interval"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PingTaskList contains configured latency probe tasks in display order.
type PingTaskList struct {
	Results []PingTaskSummary `json:"results"`
}

// PingTaskBody defines the configuration of a latency probe task.
type PingTaskBody struct {
	// Display name of the latency probe task.
	Name string `json:"name" binding:"required" minLength:"1" maxLength:"255"`
	// Probe protocol: icmp, tcp, or http.
	Type string `json:"type" binding:"required" enums:"icmp,tcp,http"`
	// Probe target address (IP or host for icmp, host:port for tcp, or full http/https URL for http).
	Target string `json:"target" binding:"required" minLength:"1" maxLength:"255"`
	// Probe interval in seconds (10 to 86400; defaults to 60).
	Interval int `json:"interval,omitempty" minimum:"10" maximum:"86400"`
	// Target node registration identifiers (node_id values from openflare_node_list).
	Clients []string `json:"clients,omitempty"`
	// Whether newly added or registered nodes are automatically bound to this task.
	DefaultOn bool `json:"default_on,omitempty"`
}

// PingTaskSaved confirms a latency probe task stored on the controller.
type PingTaskSaved struct {
	Task  PingTaskSummary `json:"task"`
	State string          `json:"state"`
}

// PingTaskDeleted confirms removal of a latency probe task.
type PingTaskDeleted struct {
	TaskID  string `json:"task_id"`
	Deleted int64  `json:"deleted"`
	State   string `json:"state"`
}

// PingTaskReorderBody specifies the desired task order by decimal task IDs.
type PingTaskReorderBody struct {
	// Ordered list of decimal ping task IDs from highest priority (weight 0) to lowest.
	TaskIDs []string `json:"task_ids" binding:"required"`
}

// NodePingMetrics combines node metadata with latency probe statistics and sampled records.
type NodePingMetrics struct {
	Node    NodeSummary              `json:"node"`
	Hours   int                      `json:"hours"`
	Tasks   []ping.PingTaskStatsView `json:"tasks"`
	Records []ping.PingRecordView    `json:"records"`
}

func summarizePingTask(item *model.OpenFlarePingTask) PingTaskSummary {
	clients := []string(item.Clients)
	if clients == nil {
		clients = []string{}
	}
	return PingTaskSummary{
		TaskID:    decimalID(item.ID),
		Weight:    item.Weight,
		Name:      item.Name,
		Clients:   clients,
		DefaultOn: item.DefaultOn,
		Type:      item.Type,
		Target:    item.Target,
		Interval:  item.Interval,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

// @ID openflare_ping_task_list
// @Summary List latency probe tasks
// @Description Return all ICMP, TCP, and HTTP latency probe tasks ordered by display weight, including bound node registration identifiers and default_on auto-binding flags. This operation is read-only.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} OperationResponse{data=PingTaskList}
// @Failure 401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ping list","scope":"ping:read","effect":"read"}
// @Router /api/v1/automation/ping-tasks [get]
func listPingTasks(ctx context.Context, _ identity, _ map[string]string) (any, *operationError) {
	items, err := ping.ListTasks(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list ping tasks", err)
	}
	results := make([]PingTaskSummary, 0, len(items))
	for _, item := range items {
		results = append(results, summarizePingTask(item))
	}
	return PingTaskList{Results: results}, nil
}

// @ID openflare_ping_task_create
// @Summary Create a latency probe task
// @Description Create an ICMP, TCP, or HTTP latency probe task targeting the specified edge nodes and immediately reload the controller probe scheduler. Use node_id values from openflare_node_list in clients.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param body body PingTaskBody true "Latency probe task configuration"
// @Success 200 {object} OperationResponse{data=PingTaskSaved}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ping create","scope":"ping:write","effect":"write","parameters":{"clients":{"minItems":0,"maxItems":200}}}
// @Router /api/v1/automation/ping-tasks [post]
func createPingTask(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	var body PingTaskBody
	if err := decodeBodyFields("openflare_ping_task_create", input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	created, err := ping.CreateTask(ctx, ping.TaskInput{
		Name:      body.Name,
		Clients:   body.Clients,
		DefaultOn: body.DefaultOn,
		Type:      body.Type,
		Target:    body.Target,
		Interval:  body.Interval,
	})
	if err != nil {
		return nil, invalidArguments(err)
	}
	return PingTaskSaved{Task: summarizePingTask(created), State: "created"}, nil
}

// @ID openflare_ping_task_update
// @Summary Update a latency probe task
// @Description Replace the configuration and bound nodes of an existing latency probe task and reload the probe scheduler. Pass task_id from openflare_ping_task_list.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param task_id path string true "Decimal ping task ID from openflare_ping_task_list"
// @Param body body PingTaskBody true "Updated latency probe task configuration"
// @Success 200 {object} OperationResponse{data=PingTaskSaved}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ping update","scope":"ping:write","effect":"write","parameters":{"task_id":{"pattern":"^[1-9][0-9]{0,18}$"},"clients":{"minItems":0,"maxItems":200}}}
// @Router /api/v1/automation/ping-tasks/{task_id} [patch]
func updatePingTask(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "task_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	var body PingTaskBody
	if err := decodeBodyFields("openflare_ping_task_update", input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	if body.Interval == 0 {
		body.Interval = 60
	}
	updated, err := ping.UpdateTask(ctx, id, ping.TaskInput{
		Name:      body.Name,
		Clients:   body.Clients,
		DefaultOn: body.DefaultOn,
		Type:      body.Type,
		Target:    body.Target,
		Interval:  body.Interval,
	})
	if err != nil {
		return nil, resourceFailure(ctx, "update ping task", err)
	}
	return PingTaskSaved{Task: summarizePingTask(updated), State: "updated"}, nil
}

// @ID openflare_ping_task_delete
// @Summary Delete a latency probe task
// @Description Delete one latency probe task and purge its historical ping measurements from logstore. Pass task_id from openflare_ping_task_list.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param task_id path string true "Decimal ping task ID from openflare_ping_task_list"
// @Success 200 {object} OperationResponse{data=PingTaskDeleted}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ping delete","scope":"ping:write","effect":"write","parameters":{"task_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/ping-tasks/{task_id} [delete]
func deletePingTask(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "task_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	deleted, err := ping.DeleteTasks(ctx, []uint{id})
	if err != nil {
		return nil, resourceFailure(ctx, "delete ping task", err)
	}
	return PingTaskDeleted{TaskID: decimalID(id), Deleted: deleted, State: "deleted"}, nil
}

// @ID openflare_ping_task_reorder
// @Summary Reorder latency probe tasks
// @Description Update display weights of latency probe tasks according to the supplied ordered list of decimal task IDs.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param body body PingTaskReorderBody true "Ordered decimal ping task IDs"
// @Success 200 {object} OperationResponse{data=PingTaskList}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"ping reorder","scope":"ping:write","effect":"write","parameters":{"task_ids":{"minItems":1,"maxItems":200}}}
// @Router /api/v1/automation/ping-tasks/order [post]
func reorderPingTasks(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	var body PingTaskReorderBody
	if err := decodeBodyFields("openflare_ping_task_reorder", input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	weights := make(map[uint]int, len(body.TaskIDs))
	for idx, rawID := range body.TaskIDs {
		parsed, err := strconv.ParseUint(rawID, 10, 63)
		if err != nil || parsed == 0 || uint64(uint(parsed)) != parsed {
			return nil, invalidArguments(errors.New("task_ids must contain positive decimal ping task IDs"))
		}
		weights[uint(parsed)] = idx
	}
	if err := ping.ReorderTasks(ctx, weights); err != nil {
		return nil, resourceFailure(ctx, "reorder ping tasks", err)
	}
	items, err := ping.ListTasks(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list reordered ping tasks", err)
	}
	results := make([]PingTaskSummary, 0, len(items))
	for _, item := range items {
		results = append(results, summarizePingTask(item))
	}
	return PingTaskList{Results: results}, nil
}

// @ID openflare_node_ping_metrics
// @Summary Read latency probe statistics and history for one node
// @Description Return per-task latency statistics (min, max, avg, latest, p50, p99, packet loss percentage) and downsampled time-series records for one edge node. Resolve node_record_id with openflare_node_list.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param node_record_id path string true "Node control-plane record ID from openflare_node_list"
// @Param task_id query string false "Optional decimal ping task ID to filter by a single task"
// @Param hours query integer false "Lookback window in hours" minimum(1) maximum(720) default(1)
// @Param max_points query integer false "Maximum sampled points per task" minimum(50) maximum(2000) default(300)
// @Success 200 {object} OperationResponse{data=NodePingMetrics}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"node ping","scope":"metrics:read","effect":"read","parameters":{"node_record_id":{"pattern":"^[1-9][0-9]{0,18}$"},"task_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/nodes/{node_record_id}/ping [get]
func nodePingMetrics(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "node_record_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	var taskID uint
	if input["task_id"] != "" {
		taskID, err = recordID(input, "task_id")
		if err != nil {
			return nil, invalidArguments(err)
		}
	}
	nodeView, err := node.GetNode(ctx, id)
	if err != nil {
		return nil, resourceFailure(ctx, "read node", err)
	}
	result, err := ping.GetNodePingRecords(ctx, id, ping.NodePingRecordsQuery{
		TaskID:    taskID,
		Hours:     inputInt(input, "hours"),
		MaxPoints: inputInt(input, "max_points"),
	})
	if err != nil {
		return nil, resourceFailure(ctx, "read node ping metrics", err)
	}
	return NodePingMetrics{
		Node:    summarizeNode(nodeView),
		Hours:   result.Hours,
		Tasks:   result.Tasks,
		Records: result.Records,
	}, nil
}
