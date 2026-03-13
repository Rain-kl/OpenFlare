package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"atsflare-agent/internal/protocol"
)

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

func (c *Client) RegisterNode(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error) {
	slog.Info("http register node request", "node_id", payload.NodeID, "current_version", payload.CurrentVersion)
	resp := protocol.APIResponse[protocol.RegisterNodeResponse]{}
	if err := c.postJSON(ctx, "/api/agent/nodes/register", payload, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Message)
	}
	slog.Info("http register node response", "node_id", resp.Data.NodeID)
	return &resp.Data, nil
}

func (c *Client) Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error) {
	resp := protocol.HeartbeatAPIResponse{}
	if err := c.postJSON(ctx, "/api/agent/nodes/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Message)
	}
	return &protocol.HeartbeatResult{
		AgentSettings: resp.AgentSettings,
		ActiveConfig:  resp.ActiveConfig,
	}, nil
}

func (c *Client) GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error) {
	resp := protocol.APIResponse[protocol.ActiveConfigResponse]{}
	if err := c.getJSON(ctx, "/api/agent/config-versions/active", &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Message)
	}
	slog.Info("http get active config response", "version", resp.Data.Version, "checksum", resp.Data.Checksum, "support_files", len(resp.Data.SupportFiles))
	return &resp.Data, nil
}

func (c *Client) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	slog.Info("http report apply log request", "node_id", payload.NodeID, "version", payload.Version, "result", payload.Result)
	return c.postJSON(ctx, "/api/agent/apply-logs", payload, nil)
}

func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
	slog.Info("http client token updated")
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
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		slog.Warn("http request returned non-200", "method", req.Method, "path", req.URL.Path, "status", res.Status)
		return errors.New(res.Status)
	}
	if target == nil {
		var wrapper protocol.APIResponse[json.RawMessage]
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
