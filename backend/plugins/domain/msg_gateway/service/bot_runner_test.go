// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/service"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubChannel struct{}

func (stubChannel) Type() string                                                 { return "stub" }
func (stubChannel) Connect(context.Context) error                                { return nil }
func (stubChannel) Disconnect(context.Context) error                             { return nil }
func (stubChannel) Send(context.Context, do.Recipient, do.OutboundMessage) error { return nil }
func (stubChannel) Capabilities() do.Capability                                  { return do.Capability{Text: true} }

func TestRegisterLookup(t *testing.T) {
	service.Register("stub", func(do.ChannelConfig, service.Handler) (service.Channel, error) {
		return stubChannel{}, nil
	})
	fn, ok := service.Lookup("stub")
	require.True(t, ok)

	ch, err := fn(do.ChannelConfig{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "stub", ch.Type())
}
