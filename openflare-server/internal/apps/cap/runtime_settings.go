// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	defaultChallengeCount      = 1
	defaultChallengeSize       = 32
	defaultChallengeDifficulty = 4
	defaultChallengeTTL        = 10 * time.Minute
	defaultTokenTTL            = 20 * time.Minute
)

// RuntimeSettings is the parsed CAPTCHA runtime configuration loaded from system_configs.
type RuntimeSettings struct {
	LoginEnabled        bool
	ChallengeCount      int
	ChallengeSize       int
	ChallengeDifficulty int
	ChallengeTTL        time.Duration
	TokenTTL            time.Duration
}

var runtimeConfigKeys = []string{
	model.ConfigKeyCapLoginEnabled,
	model.ConfigKeyCapChallengeCount,
	model.ConfigKeyCapChallengeSize,
	model.ConfigKeyCapChallengeDifficulty,
	model.ConfigKeyCapChallengeTTL,
	model.ConfigKeyCapTokenTTL,
}

var runtimeConfigKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(runtimeConfigKeys))
	for _, key := range runtimeConfigKeys {
		set[key] = struct{}{}
	}
	return set
}()

type runtimeSettingsStore struct {
	snapshot     atomic.Pointer[RuntimeSettings]
	loadGroup    singleflight.Group
	listenerOnce sync.Once
}

var settingsStore = &runtimeSettingsStore{}

// IsRuntimeConfigKey reports whether a system config key affects CAPTCHA runtime settings.
func IsRuntimeConfigKey(key string) bool {
	_, ok := runtimeConfigKeySet[key]
	return ok
}

// CurrentSettings returns the cached CAPTCHA runtime settings snapshot.
func CurrentSettings(ctx context.Context) (RuntimeSettings, error) {
	return settingsStore.current(ctx)
}

// ProtectionEnabled reports whether CAPTCHA verification is required for protected routes.
func ProtectionEnabled(ctx context.Context) bool {
	settings, err := CurrentSettings(ctx)
	if err != nil {
		return false
	}
	return settings.LoginEnabled
}

// InvalidateRuntimeSettings drops the in-process CAPTCHA settings snapshot.
func InvalidateRuntimeSettings() {
	settingsStore.snapshot.Store(nil)
}

// ResetRuntimeSettingsForTest clears the CAPTCHA runtime snapshot.
func ResetRuntimeSettingsForTest() {
	InvalidateRuntimeSettings()
}

// InstallTestRuntimeSettings installs a fixed snapshot for unit tests.
func InstallTestRuntimeSettings(settings RuntimeSettings) func() {
	snapshot := settings
	settingsStore.snapshot.Store(&snapshot)
	return InvalidateRuntimeSettings
}

func (s *runtimeSettingsStore) current(ctx context.Context) (RuntimeSettings, error) {
	s.ensureInvalidationListener()

	if snapshot := s.snapshot.Load(); snapshot != nil {
		return *snapshot, nil
	}

	loaded, err, _ := s.loadGroup.Do("cap-runtime-settings", func() (any, error) {
		if snapshot := s.snapshot.Load(); snapshot != nil {
			return *snapshot, nil
		}

		settings, loadErr := loadRuntimeSettings(ctx)
		if loadErr != nil {
			return RuntimeSettings{}, loadErr
		}

		s.snapshot.Store(&settings)
		return settings, nil
	})
	if err != nil {
		return RuntimeSettings{}, err
	}

	settings, ok := loaded.(RuntimeSettings)
	if !ok {
		return RuntimeSettings{}, errors.New("cap runtime settings loader returned unexpected type")
	}
	return settings, nil
}

func loadRuntimeSettings(ctx context.Context) (RuntimeSettings, error) {
	configs, err := repository.ListSystemConfigsByKeys(ctx, runtimeConfigKeys)
	if err != nil {
		return RuntimeSettings{}, err
	}
	return parseRuntimeSettings(configs), nil
}

func parseRuntimeSettings(configs map[string]model.SystemConfig) RuntimeSettings {
	settings := RuntimeSettings{
		ChallengeCount:      defaultChallengeCount,
		ChallengeSize:       defaultChallengeSize,
		ChallengeDifficulty: defaultChallengeDifficulty,
		ChallengeTTL:        defaultChallengeTTL,
		TokenTTL:            defaultTokenTTL,
	}

	if sc, ok := configs[model.ConfigKeyCapLoginEnabled]; ok {
		if enabled, err := strconv.ParseBool(sc.Value); err == nil {
			settings.LoginEnabled = enabled
		}
	}
	if sc, ok := configs[model.ConfigKeyCapChallengeCount]; ok {
		if count, err := strconv.Atoi(sc.Value); err == nil && count > 0 {
			settings.ChallengeCount = count
		}
	}
	if sc, ok := configs[model.ConfigKeyCapChallengeSize]; ok {
		if size, err := strconv.Atoi(sc.Value); err == nil && size > 0 {
			settings.ChallengeSize = size
		}
	}
	if sc, ok := configs[model.ConfigKeyCapChallengeDifficulty]; ok {
		if difficulty, err := strconv.Atoi(sc.Value); err == nil && difficulty > 0 {
			settings.ChallengeDifficulty = difficulty
		}
	}
	if sc, ok := configs[model.ConfigKeyCapChallengeTTL]; ok {
		if ttlSeconds, err := strconv.Atoi(sc.Value); err == nil && ttlSeconds > 0 {
			settings.ChallengeTTL = time.Duration(ttlSeconds) * time.Second
		}
	}
	if sc, ok := configs[model.ConfigKeyCapTokenTTL]; ok {
		if ttlSeconds, err := strconv.Atoi(sc.Value); err == nil && ttlSeconds > 0 {
			settings.TokenTTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	return settings
}

func (s *runtimeSettingsStore) ensureInvalidationListener() {
	s.listenerOnce.Do(startRuntimeSettingsInvalidationListener)
}

func startRuntimeSettingsInvalidationListener() {
	if db.Redis == nil {
		return
	}

	go func() {
		pubsub := db.Redis.Subscribe(context.Background(), repository.SystemConfigInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		for msg := range pubsub.Channel() {
			var payload struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				InvalidateRuntimeSettings()
				continue
			}
			if payload.Key == "" || payload.Key == "*" || IsRuntimeConfigKey(payload.Key) {
				InvalidateRuntimeSettings()
			}
		}
	}()
}
