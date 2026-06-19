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
		base: edgehttp.New(baseURL, token, timeout, "X-Agent-Token"),
	}
}

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

func (c *Client) SetToken(token string) {
	c.base.SetToken(token)
}