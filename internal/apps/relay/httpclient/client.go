// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package httpclient provides an HTTP client for relay control-plane communication.
package httpclient

import (
	"context"
	"time"

	edgehttp "github.com/Rain-kl/Wavelet/internal/apps/edge/httpclient"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

// APIResponse is an alias for service.APIResponse.
type APIResponse[T any] = service.APIResponse[T]

// Client sends authenticated requests to the relay control-plane API.
type Client struct {
	base *edgehttp.Client
}

// New creates a relay HTTP client with the given base URL, token, and timeout.
func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		base: edgehttp.New(baseURL, token, timeout, "X-Agent-Token"),
	}
}

// Heartbeat sends a relay heartbeat payload to the control plane.
func (c *Client) Heartbeat(ctx context.Context, payload service.RelayHeartbeatPayload) (*service.RelayHeartbeatResponse, error) {
	resp := APIResponse[service.RelayHeartbeatResponse]{}
	if err := c.base.PostJSON(ctx, "/api/v1/relay/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// SetToken updates the authentication token used for API requests.
func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}
