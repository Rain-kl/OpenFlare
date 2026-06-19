package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/agent/protocol"
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
	slog.Debug("http register node request", "node_id", payload.NodeID, "current_version", payload.CurrentVersion)
	resp := protocol.APIResponse[protocol.RegisterNodeResponse]{}
	if err := c.postJSON(ctx, "/api/v1/agent/nodes/register", payload, &resp); err != nil {
		return nil, err
	}
	if err := apiError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	slog.Debug("http register node response", "node_id", resp.Data.NodeID)
	return &resp.Data, nil
}

func (c *Client) Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error) {
	resp := protocol.APIResponse[protocol.HeartbeatData]{}
	if err := c.postJSON(ctx, "/api/v1/agent/nodes/heartbeat", payload, &resp); err != nil {
		return nil, err
	}
	if err := apiError(resp.ErrorMsg); err != nil {
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
	if err := c.getJSON(ctx, "/api/v1/agent/config-versions/active", &resp); err != nil {
		return nil, err
	}
	if err := apiError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	slog.Debug("http get active config response", "version", resp.Data.Version, "checksum", resp.Data.Checksum, "support_files", len(resp.Data.SupportFiles))
	return &resp.Data, nil
}

func (c *Client) ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error {
	slog.Debug("http report apply log request", "node_id", payload.NodeID, "version", payload.Version, "result", payload.Result)
	resp := protocol.APIResponse[json.RawMessage]{}
	if err := c.postJSON(ctx, "/api/v1/agent/apply-logs", payload, &resp); err != nil {
		return err
	}
	return apiError(resp.ErrorMsg)
}

func (c *Client) SyncWAFIPGroups(ctx context.Context, payload protocol.WAFIPGroupSyncRequest) (*protocol.WAFIPGroupSyncResponse, error) {
	resp := protocol.APIResponse[protocol.WAFIPGroupSyncResponse]{}
	if err := c.postJSON(ctx, "/api/v1/agent/waf/ip-groups/sync", payload, &resp); err != nil {
		return nil, err
	}
	if err := apiError(resp.ErrorMsg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DownloadPagesDeploymentPackage(ctx context.Context, deploymentID uint) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf("/api/v1/agent/pages/deployments/%d/package", deploymentID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Agent-Token", c.token)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, readHTTPError(res)
	}
	return io.ReadAll(res.Body)
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

func readHTTPError(res *http.Response) error {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.New(res.Status)
	}
	return readBodyError(body, res.Status)
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
