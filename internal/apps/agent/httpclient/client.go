// Package httpclient provides an authenticated HTTP client for the agent.
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	edgehttp "github.com/Rain-kl/Wavelet/internal/apps/edge/httpclient"
)

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

// DownloadPagesDeploymentPackage downloads the deployment package for the given Pages deployment ID.
func (c *Client) DownloadPagesDeploymentPackage(ctx context.Context, deploymentID uint) ([]byte, error) {
	res, err := c.base.DoRaw(ctx, http.MethodGet, fmt.Sprintf("/api/v1/agent/pages/deployments/%d/package", deploymentID), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, edgehttp.ReadHTTPError(res)
	}
	return io.ReadAll(res.Body)
}

// GetPagesProjectLatestHash returns the active deployment package hash for a Pages project.
func (c *Client) GetPagesProjectLatestHash(ctx context.Context, projectID uint) (*protocol.PagesProjectLatestHashResponse, error) {
	resp := protocol.APIResponse[protocol.PagesProjectLatestHashResponse]{}
	if err := c.base.GetJSON(ctx, fmt.Sprintf("/api/v1/agent/pages/projects/%d/latest/hash", projectID), &resp); err != nil {
		return nil, err
	}
	if err := edgehttp.APIError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DownloadPagesProjectLatestPackage downloads the active deployment package for a Pages project.
func (c *Client) DownloadPagesProjectLatestPackage(ctx context.Context, projectID uint) ([]byte, error) {
	res, err := c.base.DoRaw(ctx, http.MethodGet, fmt.Sprintf("/api/v1/agent/pages/projects/%d/latest/package", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, edgehttp.ReadHTTPError(res)
	}
	return io.ReadAll(res.Body)
}

// SetToken updates the authentication token used for API requests.
func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}
