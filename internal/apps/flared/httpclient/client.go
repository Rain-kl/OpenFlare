// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package httpclient provides the HTTP client used by the flared agent to communicate with the Wavelet server.
package httpclient

import (
	"context"
	"time"

	edgehttp "github.com/Rain-kl/Wavelet/internal/apps/edge/httpclient"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

// APIResponse is an alias for service.APIResponse.
type APIResponse[T any] = service.APIResponse[T]

// Client is the HTTP client for the flared tunnel API.
type Client struct {
	base *edgehttp.Client
}

// New creates a new Client configured with the given base URL, authentication token, and request timeout.
func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		base: edgehttp.New(baseURL, token, timeout, "X-Tunnel-Token"),
	}
}

// Heartbeat sends a tunnel heartbeat payload and returns the server response.
func (c *Client) Heartbeat(ctx context.Context, payload service.FlaredHeartbeatPayload) (*service.FlaredHeartbeatResponse, error) {
	resp := APIResponse[service.FlaredHeartbeatResponse]{}
	if err := c.base.PostJSON(ctx, "/api/v1/tunnel/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetActiveConfig fetches the currently active tunnel configuration from the server.
func (c *Client) GetActiveConfig(ctx context.Context) (*service.FlaredTunnelConfigResponse, error) {
	resp := APIResponse[service.FlaredTunnelConfigResponse]{}
	if err := c.base.GetJSON(ctx, "/api/v1/tunnel/config/active", &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// ReportApplyLog submits a configuration apply-log entry to the server.
func (c *Client) ReportApplyLog(ctx context.Context, payload service.ApplyLogPayload) error {
	resp := APIResponse[any]{}
	if err := c.base.PostJSON(ctx, "/api/v1/tunnel/apply-log", payload, &resp); err != nil {
		return err
	}
	return edgehttp.APIError(resp.ErrorMsg)
}

// SetToken updates the authentication token used by the client.
func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}
