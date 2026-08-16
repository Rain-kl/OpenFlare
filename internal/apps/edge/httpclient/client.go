// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package httpclient provides an authenticated HTTP client for edge services.
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
)

// Client is an HTTP client wrapper for communicating with remote HTTP services.
type Client struct {
	baseURL    string
	token      string
	authHeader string
	httpClient *http.Client
}

// New creates a new Client instance.
func New(baseURL, token string, timeout time.Duration, authHeader string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		authHeader: authHeader,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// SetToken updates the client auth token.
func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
	slog.Debug("http client token updated")
}

// GetJSON sends a GET request and decodes the response body into target.
func (c *Client) GetJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.setAuthHeader(req)
	return c.do(req, target)
}

// PostJSON sends a POST request with JSON body and decodes the response body into target.
func (c *Client) PostJSON(ctx context.Context, path string, body any, target any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	return c.do(req, target)
}

// DoRaw performs an HTTP request with custom headers and returns the raw response.
func (c *Client) DoRaw(ctx context.Context, method, path string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return c.httpClient.Do(req)
}

func (c *Client) setAuthHeader(req *http.Request) {
	if c.authHeader != "" {
		req.Header.Set(c.authHeader, c.token)
	}
}

func (c *Client) do(req *http.Request, target any) error {
	res, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("http request failed", "method", req.Method, "path", req.URL.Path, "error", err)
		return err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("http response read failed", "method", req.Method, "path", req.URL.Path, "error", err)
		return err
	}
	if res.StatusCode != http.StatusOK {
		slog.Warn("http request returned non-200", "method", req.Method, "path", req.URL.Path, "status", res.Status)
		return ReadBodyError(body, res.Status)
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

// APIError creates a new API error with the given message if it is not empty.
func APIError(msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}
	return errors.New(msg)
}

// ReadBodyError parses the error message from the response body, or returns the fallback message.
func ReadBodyError(body []byte, fallback string) error {
	var errBody struct {
		ErrorMsg string `json:"error_msg"`
	}
	if err := json.Unmarshal(body, &errBody); err == nil && strings.TrimSpace(errBody.ErrorMsg) != "" {
		return errors.New(errBody.ErrorMsg)
	}
	return errors.New(fallback)
}

// ReadHTTPError reads the error message from the HTTP response.
func ReadHTTPError(res *http.Response) error {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.New(res.Status)
	}
	return ReadBodyError(body, res.Status)
}
