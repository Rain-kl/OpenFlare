package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	edgehttp "github.com/Rain-kl/Wavelet/internal/apps/edge/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

type Client struct {
	base *edgehttp.Client
}

func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		base: edgehttp.New(baseURL, token, timeout, "X-Agent-Token"),
	}
}

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

func (c *Client) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	resp := protocol.APIResponse[json.RawMessage]{}
	if err := c.base.PostJSON(ctx, "/api/v1/agent/apply-logs", payload, &resp); err != nil {
		return err
	}
	return edgehttp.APIError(resp.ErrorMsg)
}

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

func (c *Client) DownloadPagesDeploymentPackage(ctx context.Context, deploymentID uint) ([]byte, error) {
	res, err := c.base.DoRaw(ctx, http.MethodGet, fmt.Sprintf("/api/v1/agent/pages/deployments/%d/package", deploymentID), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, edgehttp.ReadHTTPError(res)
	}
	return io.ReadAll(res.Body)
}

func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}