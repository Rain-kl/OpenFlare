// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ping

const (
	errPingTaskNotFound        = "ping task was not found"
	errPingTaskNameRequired    = "ping task name must not be empty"
	errPingTaskNameTooLong     = "ping task name must not exceed 255 characters"
	errPingTaskTypeInvalid     = "ping task type must be icmp, tcp, or http"
	errPingTaskTargetRequired  = "ping task target must not be empty"
	errPingTaskTargetTooLong   = "ping task target must not exceed 255 characters"
	errPingTaskIntervalInvalid = "ping task interval must be between 10 and 86400 seconds"
	errPingTaskIDsRequired     = "at least one ping task id is required"
	errPingTaskBatchRequired   = "at least one ping task update is required"
	errPingTaskWeightsRequired = "at least one ping task weight is required"
	errPingTaskWeightIDInvalid = "invalid ping task id in order map"
	errPingResultInvalid       = "invalid ping result payload"
	errNodeNotFound            = "node was not found"
)
