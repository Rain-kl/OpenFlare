// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"strings"

	"github.com/hibiken/asynq"
)

const defaultPermanentErrorMessage = "任务无法继续执行"

type permanentTaskError struct {
	message string
}

// PermanentError marks a safe domain message as a non-retryable task failure.
// It intentionally accepts no underlying error so Error never exposes provider,
// URL, header, response-body, or other sensitive implementation details.
func PermanentError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = defaultPermanentErrorMessage
	}
	return &permanentTaskError{message: message}
}

func (e *permanentTaskError) Error() string {
	return e.message
}

func (e *permanentTaskError) Unwrap() error {
	return asynq.SkipRetry
}
