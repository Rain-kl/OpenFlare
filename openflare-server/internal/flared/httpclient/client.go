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

	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

type APIResponse[T any] struct {
	ErrorMsg string `json:"error_msg"`
	Data     T      `json:"data"`
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

func (c *Client) Heartbeat(ctx context.Context, payload service.FlaredHeartbeatPayload) (*service.FlaredHeartbeatResponse, error) {
	resp := APIResponse[service.FlaredHeartbeatResponse]{}
	if err := c.postJSON(ctx, "/api/v1/tunnel/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if err := apiError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetActiveConfig(ctx context.Context) (*service.FlaredTunnelConfigResponse, error) {
	resp := APIResponse[service.FlaredTunnelConfigResponse]{}
	if err := c.getJSON(ctx, "/api/v1/tunnel/config/active", &resp); err != nil {
		return nil, err
	}
	if err := apiError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ReportApplyLog(ctx context.Context, payload service.ApplyLogPayload) error {
	resp := APIResponse[any]{}
	if err := c.postJSON(ctx, "/api/v1/tunnel/apply-log", payload, &resp); err != nil {
		return err
	}
	return apiError(resp.ErrorMsg)
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
	req.Header.Set("X-Tunnel-Token", c.token)
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
	req.Header.Set("X-Tunnel-Token", c.token)
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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("http response read failed", "method", req.Method, "path", req.URL.Path, "error", err)
		return err
	}
	if res.StatusCode != http.StatusOK {
		slog.Warn("http request returned non-200", "method", req.Method, "path", req.URL.Path, "status", res.Status)
		return readBodyError(body, res.Status)
	}
	if target == nil {
		return nil
	}
	if err = json.Unmarshal(body, target); err != nil {
		slog.Error("http response decode failed", "method", req.Method, "path", req.URL.Path, "error", err)
		return err
	}
	return nil
}

func apiError(msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}
	return errors.New(msg)
}

func readBodyError(body []byte, fallback string) error {
	var errBody struct {
		ErrorMsg string `json:"error_msg"`
	}
	if err := json.Unmarshal(body, &errBody); err == nil && strings.TrimSpace(errBody.ErrorMsg) != "" {
		return errors.New(errBody.ErrorMsg)
	}
	return errors.New(fallback)
}
