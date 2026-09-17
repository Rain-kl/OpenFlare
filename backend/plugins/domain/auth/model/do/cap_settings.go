// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package do provides domain data objects for the auth plugin.
package do

import (
	"Wavelet/plugins/domain/auth/consts"
	"strconv"
	"time"
)

// CapRuntimeSettings is the parsed CAPTCHA runtime configuration loaded from system_configs.
type CapRuntimeSettings struct {
	LoginEnabled        bool
	ChallengeCount      int
	ChallengeSize       int
	ChallengeDifficulty int
	ChallengeTTL        time.Duration
	TokenTTL            time.Duration
}

// CapConfigRecord maps the columns selected from the system config table.
type CapConfigRecord struct {
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:value"`
}

// ParseCapRuntimeSettings parses system config key-value map into CapRuntimeSettings with fallback defaults.
func ParseCapRuntimeSettings(configs map[string]string) CapRuntimeSettings {
	settings := CapRuntimeSettings{
		ChallengeCount:      consts.DefaultCapChallengeCount,
		ChallengeSize:       consts.DefaultCapChallengeSize,
		ChallengeDifficulty: consts.DefaultCapChallengeDifficulty,
		ChallengeTTL:        consts.DefaultCapChallengeTTL,
		TokenTTL:            consts.DefaultCapTokenTTL,
	}

	if len(configs) == 0 {
		return settings
	}

	if val, ok := configs[consts.ConfigKeyCapLoginEnabled]; ok {
		if enabled, err := strconv.ParseBool(val); err == nil {
			settings.LoginEnabled = enabled
		}
	}
	if val, ok := configs[consts.ConfigKeyCapChallengeCount]; ok {
		if count, err := strconv.Atoi(val); err == nil && count > 0 {
			settings.ChallengeCount = count
		}
	}
	if val, ok := configs[consts.ConfigKeyCapChallengeSize]; ok {
		if size, err := strconv.Atoi(val); err == nil && size > 0 {
			settings.ChallengeSize = size
		}
	}
	if val, ok := configs[consts.ConfigKeyCapChallengeDifficulty]; ok {
		if diff, err := strconv.Atoi(val); err == nil && diff > 0 {
			settings.ChallengeDifficulty = diff
		}
	}
	if val, ok := configs[consts.ConfigKeyCapChallengeTTL]; ok {
		if ttlSeconds, err := strconv.Atoi(val); err == nil && ttlSeconds > 0 {
			settings.ChallengeTTL = time.Duration(ttlSeconds) * time.Second
		}
	}
	if val, ok := configs[consts.ConfigKeyCapTokenTTL]; ok {
		if ttlSeconds, err := strconv.Atoi(val); err == nil && ttlSeconds > 0 {
			settings.TokenTTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	return settings
}
