// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/exec"
)

// ExecRunBody specifies a remote shell command execution request.
type ExecRunBody struct {
	// Shell command to execute on the selected edge nodes.
	Command string `json:"command" binding:"required" minLength:"1" maxLength:"32768"`
	// Target node registration identifiers (node_id values from openflare_node_list).
	Clients []string `json:"clients" binding:"required"`
	// Per-node execution timeout in seconds (1 to 600; defaults to 60).
	TimeoutSeconds int `json:"timeout_seconds,omitempty" minimum:"1" maximum:"600"`
}

// ExecTaskList wraps recent remote execution tasks.
type ExecTaskList struct {
	Results []*exec.TaskView `json:"results"`
}

// @ID openflare_exec_run
// @Summary Dispatch a remote shell command to edge nodes
// @Description Create a remote execution task and dispatch the shell command over WebSocket to the selected online edge nodes. Poll completion with openflare_exec_get using the returned task_id. Pass node_id values from openflare_node_list in clients.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param body body ExecRunBody true "Remote command execution request"
// @Success 200 {object} OperationResponse{data=exec.RunResponse}
// @Failure 400,401,404,500 {object} OperationResponse
// @Failure 403 {object} OperationResponse "Insufficient permissions or remote execution disabled at startup"
// @x-automation {"cli":"exec run","scope":"exec:write","effect":"write","parameters":{"clients":{"minItems":1,"maxItems":200}}}
// @Router /api/v1/automation/exec [post]
func runRemoteExec(ctx context.Context, actor identity, input map[string]string) (any, *operationError) {
	var body ExecRunBody
	if err := decodeBodyFields("openflare_exec_run", input, &body); err != nil {
		return nil, invalidArguments(err)
	}
	res, err := exec.RunCommand(ctx, uint(actor.userID), exec.RunInput{
		Command:        body.Command,
		Clients:        body.Clients,
		TimeoutSeconds: body.TimeoutSeconds,
	})
	if errors.Is(err, exec.ErrWebShellDisabled) {
		return nil, &operationError{http.StatusForbidden, "WEBSHELL_DISABLED", err.Error()}
	}
	if err != nil {
		return nil, invalidArguments(err)
	}
	return res, nil
}

// @ID openflare_exec_list
// @Summary List recent remote command execution tasks
// @Description Return recent remote execution tasks together with their per-node exit codes and outputs. This operation is read-only.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param limit query integer false "Maximum number of tasks to return" minimum(1) maximum(200) default(50)
// @Success 200 {object} OperationResponse{data=ExecTaskList}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"exec list","scope":"exec:read","effect":"read"}
// @Router /api/v1/automation/exec/tasks [get]
func listRemoteExecTasks(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	tasks, err := exec.ListTasks(ctx, inputInt(input, "limit"))
	if err != nil {
		return nil, operationFailed(ctx, "list remote execution tasks", err)
	}
	return ExecTaskList{Results: tasks}, nil
}

// @ID openflare_exec_get
// @Summary Read one remote command execution task and per-node outputs
// @Description Return one remote execution task by task_id with each target node's output, exit code, and completion timestamp. Use this operation to poll tasks started by openflare_exec_run.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param task_id path string true "Remote execution task identifier returned by openflare_exec_run or openflare_exec_list"
// @Success 200 {object} OperationResponse{data=exec.TaskView}
// @Failure 400,401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"exec get","scope":"exec:read","effect":"read","parameters":{"task_id":{"pattern":"^[A-Za-z0-9_-]{4,64}$"}}}
// @Router /api/v1/automation/exec/tasks/{task_id} [get]
func getRemoteExecTask(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	task, err := exec.GetTask(ctx, input["task_id"])
	if err != nil {
		return nil, resourceFailure(ctx, "get remote execution task", err)
	}
	return task, nil
}
