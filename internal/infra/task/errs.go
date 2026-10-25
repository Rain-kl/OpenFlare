// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

// RetryError identifies a rejected retry without depending on the message language.
type RetryError struct {
	Err      error
	NotFound bool
}

func (e *RetryError) Error() string { return e.Err.Error() }
func (e *RetryError) Unwrap() error { return e.Err }

const (
	errUnknownTaskType            = "unknown task type: %s"
	errCreateTaskExecutionFailed  = "failed to create task execution record: %w"
	errTaskEnqueueFailed          = "failed to enqueue task: %w"
	errTaskExecutionNotFound      = "task execution was not found: %w"
	errRetryOnlyFailedTask        = "only failed tasks can be retried; current status: %s"
	errTaskNotRetryable           = "this task cannot be retried"
	errTaskMaxRetryExceeded       = "Maximum retry count %d reached"
	errCreateRetryExecutionFailed = "failed to create retry execution record: %w"
	errRetryTaskEnqueueFailed     = "failed to enqueue retry task: %w"
	errUnregisteredTaskHandler    = "unregistered task handler: %s"
)
