// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/model/do"
	"context"
	"errors"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
)

var capRuntimeConfigKeys = []string{
	consts.ConfigKeyCapLoginEnabled,
	consts.ConfigKeyCapChallengeCount,
	consts.ConfigKeyCapChallengeSize,
	consts.ConfigKeyCapChallengeDifficulty,
	consts.ConfigKeyCapChallengeTTL,
	consts.ConfigKeyCapTokenTTL,
}

var capRuntimeConfigKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(capRuntimeConfigKeys))
	for _, key := range capRuntimeConfigKeys {
		set[key] = struct{}{}
	}
	return set
}()

// IsCapRuntimeConfigKey reports whether a system config key affects CAPTCHA runtime settings.
func IsCapRuntimeConfigKey(key string) bool {
	_, ok := capRuntimeConfigKeySet[key]
	return ok
}

// CapSettingsManager manages dynamic CAPTCHA configuration cache.
type CapSettingsManager struct {
	dao       *dao.DAO
	snapshot  atomic.Pointer[do.CapRuntimeSettings]
	loadGroup singleflight.Group
}

// NewCapSettingsManager creates a new CapSettingsManager.
func NewCapSettingsManager(d *dao.DAO) *CapSettingsManager {
	return &CapSettingsManager{
		dao: d,
	}
}

// Invalidate drops the in-process CAPTCHA settings snapshot.
func (m *CapSettingsManager) Invalidate() {
	m.snapshot.Store(nil)
}

// Current returns the cached CAPTCHA runtime settings snapshot.
func (m *CapSettingsManager) Current(ctx context.Context) (do.CapRuntimeSettings, error) {
	if snapshot := m.snapshot.Load(); snapshot != nil {
		return *snapshot, nil
	}

	loaded, err, _ := m.loadGroup.Do("cap-runtime-settings", func() (any, error) {
		if snapshot := m.snapshot.Load(); snapshot != nil {
			return *snapshot, nil
		}

		settings, loadErr := m.loadSettings(ctx)
		if loadErr != nil {
			return do.CapRuntimeSettings{}, loadErr
		}

		m.snapshot.Store(&settings)
		return settings, nil
	})
	if err != nil {
		return do.CapRuntimeSettings{}, err
	}

	settings, ok := loaded.(do.CapRuntimeSettings)
	if !ok {
		return do.CapRuntimeSettings{}, errors.New("cap runtime settings loader returned unexpected type")
	}
	return settings, nil
}

// CapProtectionEnabled reports whether CAPTCHA verification is required for protected routes.
func (m *CapSettingsManager) CapProtectionEnabled(ctx context.Context) bool {
	settings, err := m.Current(ctx)
	if err != nil {
		return false
	}
	return settings.LoginEnabled
}

// InstallTestSnapshot installs a fixed snapshot for unit tests.
func (m *CapSettingsManager) InstallTestSnapshot(settings do.CapRuntimeSettings) func() {
	snapshot := settings
	m.snapshot.Store(&snapshot)
	return m.Invalidate
}

func (m *CapSettingsManager) loadSettings(ctx context.Context) (do.CapRuntimeSettings, error) {
	if m.dao == nil {
		return do.ParseCapRuntimeSettings(nil), nil
	}
	records, err := m.dao.ListSystemConfigsByKeys(ctx, capRuntimeConfigKeys)
	if err != nil {
		return do.CapRuntimeSettings{}, err
	}
	configs := make(map[string]string, len(records))
	for _, r := range records {
		configs[r.Key] = r.Value
	}
	return do.ParseCapRuntimeSettings(configs), nil
}
