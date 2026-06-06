package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rain-kl/openflare/openflare-server/service"
)

type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Heartbeat(ctx context.Context, payload service.RelayHeartbeatPayload) (*service.RelayHeartbeatResponse, error) {
	resp := APIResponse[service.RelayHeartbeatResponse]{}
	if err := c.postJSON(ctx, "/api/relay/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Message)
	}
	return &resp.Data, nil
}

func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
	slog.Debug("http client token updated")
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Agent-Token", c.token)
	return c.do(req, target)
}

func (c *Client) postJSON(ctx context.Context, path string, body any, target any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", c.token)
	return c.do(req, target)
}

func (c *Client) do(req *http.Request, target any) error {
	res, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("http request failed", "method", req.Method, "path", req.URL.Path, "error", err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}(res.Body)
	if res.StatusCode != http.StatusOK {
		slog.Warn("http request returned non-200", "method", req.Method, "path", req.URL.Path, "status", res.Status)
		return errors.New(res.Status)
	}
	if target == nil {
		var wrapper APIResponse[json.RawMessage]
		if err = json.NewDecoder(res.Body).Decode(&wrapper); err != nil {
			slog.Error("http response decode failed", "method", req.Method, "path", req.URL.Path, "error", err)
			return err
		}
		if !wrapper.Success {
			slog.Warn("http api response failed", "method", req.Method, "path", req.URL.Path, "message", wrapper.Message)
			return errors.New(wrapper.Message)
		}
		return nil
	}
	if err = json.NewDecoder(res.Body).Decode(target); err != nil {
		slog.Error("http response decode failed", "method", req.Method, "path", req.URL.Path, "error", err)
		return err
	}
	return nil
}
