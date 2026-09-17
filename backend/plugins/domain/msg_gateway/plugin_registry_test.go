// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package msg_gateway_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/msg_gateway"
	"Wavelet/plugins/domain/msg_gateway/service"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushRegistry(t *testing.T) {
	ctx := core.NewContext(context.Background())
	require.NoError(t, msg_gateway.New().Apply(ctx))

	registry, err := core.Inject[contracts.PushRegistry](ctx)
	require.NoError(t, err)
	require.NotNil(t, registry)

	const key = "test.push_registry.probe"
	registry.RegisterBuiltInEvent(contracts.PushEventMeta{
		Key:         key,
		Name:        "Push Registry Probe",
		Description: "observability probe for contracts.PushRegistry",
		DefaultTemplate: contracts.PushNotificationTemplate{
			Title:   "Probe Title",
			Content: "Probe Content",
			Level:   "INFO",
			Ext:     map[string]any{"source": "test"},
		},
	})

	found := false
	for _, ev := range service.GetBuiltInEvents() {
		if ev.Key != key {
			continue
		}
		found = true
		assert.Equal(t, "Push Registry Probe", ev.Name)
		assert.Equal(t, "observability probe for contracts.PushRegistry", ev.Description)
		assert.Equal(t, "Probe Title", ev.DefaultTemplate.Title)
		assert.Equal(t, "Probe Content", ev.DefaultTemplate.Content)
		assert.Equal(t, "INFO", ev.DefaultTemplate.Level)
		assert.Equal(t, map[string]any{"source": "test"}, ev.DefaultTemplate.Ext)
		break
	}
	require.True(t, found, "registered key %q should be visible via GetBuiltInEvents", key)
}
