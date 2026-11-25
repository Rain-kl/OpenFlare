// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package exec

import "errors"

// ErrWebShellDisabled is returned when startup configuration disables command dispatch.
var ErrWebShellDisabled = errors.New(errExecDisabled)

const (
	errExecDisabled         = "remote execution is disabled on this server"
	errExecCommandRequired  = "command must not be empty"
	errExecCommandTooLong   = "command exceeds maximum allowed length"
	errExecClientsRequired  = "at least one target node must be selected"
	errExecNoValidClients   = "none of the specified target nodes exist"
	errExecTaskIDRequired   = "task_id must not be empty"
	errExecTaskNotFound     = "remote execution task was not found"
	errExecResultInvalid    = "invalid remote execution result payload"
	errExecNodeQuarantined  = "node is quarantined"
	errExecNodeOffline      = "node is offline"
	errExecNodeDisconnected = "node is not connected via WebSocket"
)
