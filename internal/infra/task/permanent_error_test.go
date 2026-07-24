// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
)

func TestPermanentErrorSkipsRetryWithoutExposingAsynqMessage(t *testing.T) {
	err := PermanentError("  来源配置无效  ")

	assert.True(t, errors.Is(err, asynq.SkipRetry))
	assert.Equal(t, "来源配置无效", err.Error())
	assert.NotContains(t, err.Error(), asynq.SkipRetry.Error())
}

func TestPermanentErrorUsesSafeFallbackForBlankMessage(t *testing.T) {
	err := PermanentError("  ")

	assert.True(t, errors.Is(err, asynq.SkipRetry))
	assert.Equal(t, defaultPermanentErrorMessage, err.Error())
}
