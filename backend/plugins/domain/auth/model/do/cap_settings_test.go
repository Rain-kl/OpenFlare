// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package do_test

import (
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/model/do"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseCapRuntimeSettings(t *testing.T) {
	t.Run("Default fallback on empty config", func(t *testing.T) {
		settings := do.ParseCapRuntimeSettings(nil)
		assert.False(t, settings.LoginEnabled)
		assert.Equal(t, consts.DefaultCapChallengeCount, settings.ChallengeCount)
		assert.Equal(t, consts.DefaultCapChallengeSize, settings.ChallengeSize)
		assert.Equal(t, consts.DefaultCapChallengeDifficulty, settings.ChallengeDifficulty)
		assert.Equal(t, consts.DefaultCapChallengeTTL, settings.ChallengeTTL)
		assert.Equal(t, consts.DefaultCapTokenTTL, settings.TokenTTL)
	})

	t.Run("Parsed custom configs", func(t *testing.T) {
		configs := map[string]string{
			consts.ConfigKeyCapLoginEnabled:        "true",
			consts.ConfigKeyCapChallengeCount:      "3",
			consts.ConfigKeyCapChallengeSize:       "64",
			consts.ConfigKeyCapChallengeDifficulty: "5",
			consts.ConfigKeyCapChallengeTTL:        "300",
			consts.ConfigKeyCapTokenTTL:            "600",
		}
		settings := do.ParseCapRuntimeSettings(configs)
		assert.True(t, settings.LoginEnabled)
		assert.Equal(t, 3, settings.ChallengeCount)
		assert.Equal(t, 64, settings.ChallengeSize)
		assert.Equal(t, 5, settings.ChallengeDifficulty)
		assert.Equal(t, 300*time.Second, settings.ChallengeTTL)
		assert.Equal(t, 600*time.Second, settings.TokenTTL)
	})
}
