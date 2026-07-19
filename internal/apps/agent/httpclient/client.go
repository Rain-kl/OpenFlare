// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package httpclient provides an authenticated HTTP client for the agent.
package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	edgehttp "github.com/Rain-kl/Wavelet/internal/apps/edge/httpclient"
)

const pagesControlResponseMaxBytes = int64(64 * 1024)

// Client is a HTTP client used by the agent to communicate with the control plane server.
type Client struct {
	base *edgehttp.Client
}

// New creates a new Client instance with the specified base URL, token, and timeout.
func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		base: edgehttp.New(baseURL, token, timeout, "X-Agent-Token"),
	}
}

// RegisterNode registers the agent node with the control plane server.
func (c *Client) RegisterNode(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error) {
	resp := protocol.APIResponse[protocol.RegisterNodeResponse]{}
	if err := c.base.PostJSON(ctx, "/api/v1/agent/nodes/register", payload, &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Heartbeat sends a heartbeat payload to the control plane and returns the response result.
func (c *Client) Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error) {
	resp := protocol.APIResponse[protocol.HeartbeatData]{}
	if err := c.base.PostJSON(ctx, "/api/v1/agent/nodes/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &protocol.HeartbeatResult{
		AgentSettings: resp.Data.AgentSettings,
		ActiveConfig:  resp.Data.ActiveConfig,
		WAFIPGroups:   resp.Data.WAFIPGroups,
	}, nil
}

// GetActiveConfig retrieves the current active configuration from the control plane server.
func (c *Client) GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error) {
	resp := protocol.APIResponse[protocol.ActiveConfigResponse]{}
	if err := c.base.GetJSON(ctx, "/api/v1/agent/config-versions/active", &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// ReportApplyLog reports the configuration application logs back to the control plane.
func (c *Client) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	resp := protocol.APIResponse[json.RawMessage]{}
	if err := c.base.PostJSON(ctx, "/api/v1/agent/apply-logs", payload, &resp); err != nil {
		return err
	}
	return edgehttp.APIError(resp.ErrorMsg)
}

// SyncWAFIPGroups synchronizes WAF IP groups with the control plane server.
func (c *Client) SyncWAFIPGroups(ctx context.Context, payload protocol.WAFIPGroupSyncRequest) (*protocol.WAFIPGroupSyncResponse, error) {
	resp := protocol.APIResponse[protocol.WAFIPGroupSyncResponse]{}
	if err := c.base.PostJSON(ctx, "/api/v1/agent/waf/ip-groups/sync", payload, &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetPagesDeploymentHash returns the upload SHA-256 hash for the given Pages deployment ID.
func (c *Client) GetPagesDeploymentHash(ctx context.Context, deploymentID uint) (string, error) {
	resp := protocol.APIResponse[protocol.PagesDeploymentHashResponse]{}
	if err := c.base.GetJSON(ctx, fmt.Sprintf("/api/v1/agent/pages/deployments/%d/hash", deploymentID), &resp); err != nil {
		return "", err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return "", err
	}
	return resp.Data.Hash, nil
}

// DownloadPagesDeploymentPackage streams the deployment package into dst while
// enforcing maxBytes against both advertised and actual response sizes.
func (c *Client) DownloadPagesDeploymentPackage(
	ctx context.Context,
	deploymentID uint,
	dst io.Writer,
	maxBytes int64,
) (int64, error) {
	return c.downloadPagesPackage(
		ctx,
		fmt.Sprintf("/api/v1/agent/pages/deployments/%d/package", deploymentID),
		dst,
		maxBytes,
	)
}

// GetPagesProjectLatestHash returns the active deployment package hash for a Pages project.
func (c *Client) GetPagesProjectLatestHash(ctx context.Context, projectID uint) (*protocol.PagesProjectLatestHashResponse, error) {
	res, err := c.base.DoRaw(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/api/v1/agent/pages/projects/%d/latest/hash", projectID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := readPagesControlResponse(res, pagesControlResponseMaxBytes)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, edgehttp.ReadBodyError(body, res.Status)
	}
	resp := protocol.APIResponse[protocol.PagesProjectLatestHashResponse]{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DownloadPagesProjectLatestPackage streams the active deployment package into
// dst while enforcing maxBytes against both advertised and actual sizes.
func (c *Client) DownloadPagesProjectLatestPackage(
	ctx context.Context,
	projectID uint,
	dst io.Writer,
	maxBytes int64,
) (int64, error) {
	return c.downloadPagesPackage(
		ctx,
		fmt.Sprintf("/api/v1/agent/pages/projects/%d/latest/package", projectID),
		dst,
		maxBytes,
	)
}

func (c *Client) downloadPagesPackage(
	ctx context.Context,
	path string,
	dst io.Writer,
	maxBytes int64,
) (int64, error) {
	if dst == nil {
		return 0, errors.New("pages package destination is required")
	}
	if maxBytes <= 0 {
		return 0, errors.New("pages package byte limit must be positive")
	}
	res, err := c.base.DoRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, readErr := readPagesControlResponse(res, pagesControlResponseMaxBytes)
		if readErr != nil {
			return 0, readErr
		}
		return 0, edgehttp.ReadBodyError(body, res.Status)
	}
	return copyPagesPackageResponse(dst, res, maxBytes)
}

func readPagesControlResponse(res *http.Response, maxBytes int64) ([]byte, error) {
	if res.ContentLength > maxBytes {
		return nil, fmt.Errorf(
			"pages control response Content-Length %d exceeds limit %d",
			res.ContentLength,
			maxBytes,
		)
	}
	limited := &io.LimitedReader{R: res.Body, N: maxBytes + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read pages control response: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("pages control response body exceeds limit %d", maxBytes)
	}
	return body, nil
}

func copyPagesPackageResponse(dst io.Writer, res *http.Response, maxBytes int64) (int64, error) {
	if res.ContentLength > maxBytes {
		return 0, fmt.Errorf(
			"pages package Content-Length %d exceeds limit %d",
			res.ContentLength,
			maxBytes,
		)
	}

	limited := &io.LimitedReader{R: res.Body, N: maxBytes + 1}
	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, fmt.Errorf("stream pages package: %w", err)
	}
	if written > maxBytes {
		return written, fmt.Errorf("pages package body exceeds limit %d", maxBytes)
	}
	return written, nil
}

// SetToken updates the authentication token used for API requests.
func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}
