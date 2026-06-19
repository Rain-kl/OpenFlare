package httpclient

import (
	"context"
	"time"

	edgehttp "github.com/Rain-kl/Wavelet/internal/apps/edge/httpclient"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

type APIResponse[T any] struct {
	ErrorMsg string `json:"error_msg"`
	Data     T      `json:"data"`
}

type Client struct {
	base *edgehttp.Client
}

func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		base: edgehttp.New(baseURL, token, timeout, "X-Tunnel-Token"),
	}
}

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

func (c *Client) ReportApplyLog(ctx context.Context, payload service.ApplyLogPayload) error {
	resp := APIResponse[any]{}
	if err := c.base.PostJSON(ctx, "/api/v1/tunnel/apply-log", payload, &resp); err != nil {
		return err
	}
	return edgehttp.APIError(resp.ErrorMsg)
}

func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}