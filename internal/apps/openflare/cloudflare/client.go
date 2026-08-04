// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cloudflare manages Cloudflare DNS pointing for OpenFlare domains.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
)

const (
	defaultAPIBaseURL    = "https://api.cloudflare.com/client/v4"
	defaultHTTPTimeout   = 20 * time.Second
	maxRequestAttempts   = 3
	maxResponseBodyBytes = 1 << 20
	defaultRetryDelay    = 200 * time.Millisecond
	maxRetryAfterSeconds = 2
)

// Zone is a Cloudflare DNS zone.
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DNSRecord is a Cloudflare DNS record.
type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

// RecordInput is the desired Cloudflare DNS record payload.
type RecordInput struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

// Client describes the Cloudflare operations used by pointing reconciliation.
type Client interface {
	VerifyToken(context.Context) error
	FindZone(context.Context, string) (*Zone, error)
	GetRecord(context.Context, string, string) (*DNSRecord, error)
	ListARecords(context.Context, string, string) ([]DNSRecord, error)
	CreateARecord(context.Context, string, RecordInput) (*DNSRecord, error)
	UpdateARecord(context.Context, string, string, RecordInput) (*DNSRecord, error)
	DeleteRecord(context.Context, string, string) error
}

// HTTPClient implements Client with Cloudflare's v4 HTTP API.
type HTTPClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures HTTPClient.
type ClientOption func(*HTTPClient)

// WithBaseURL overrides the Cloudflare API base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(client *HTTPClient) { client.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithHTTPClient overrides the HTTP transport.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *HTTPClient) { client.httpClient = httpClient }
}

// NewHTTPClient creates a Cloudflare HTTP client.
func NewHTTPClient(token string, options ...ClientOption) *HTTPClient {
	client := &HTTPClient{
		token:      strings.TrimSpace(token),
		baseURL:    defaultAPIBaseURL,
		httpClient: httppool.NewClient(defaultHTTPTimeout),
	}
	for _, option := range options {
		option(client)
	}
	return client
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiEnvelope[T any] struct {
	Success bool       `json:"success"`
	Errors  []apiError `json:"errors"`
	Result  T          `json:"result"`
}

// VerifyToken verifies that the configured API token is active.
func (client *HTTPClient) VerifyToken(ctx context.Context) error {
	var result struct {
		Status string `json:"status"`
	}
	if err := client.do(ctx, http.MethodGet, "/user/tokens/verify", nil, nil, &result); err != nil {
		return err
	}
	if result.Status != "active" {
		return errors.New("cloudflare API Token 未激活")
	}
	return nil
}

// FindZone returns the exact Cloudflare zone name.
func (client *HTTPClient) FindZone(ctx context.Context, name string) (*Zone, error) {
	query := url.Values{"name": {strings.TrimSpace(name)}, "status": {"active"}, "per_page": {"2"}}
	var zones []Zone
	if err := client.do(ctx, http.MethodGet, "/zones", query, nil, &zones); err != nil {
		return nil, err
	}
	if len(zones) != 1 {
		return nil, fmt.Errorf("cloudflare 中未找到唯一 Zone %s", name)
	}
	return &zones[0], nil
}

// GetRecord returns a DNS record by ID.
func (client *HTTPClient) GetRecord(ctx context.Context, zoneID, recordID string) (*DNSRecord, error) {
	var record DNSRecord
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(recordID)
	if err := client.do(ctx, http.MethodGet, path, nil, nil, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// ListARecords lists exact-name A records.
func (client *HTTPClient) ListARecords(ctx context.Context, zoneID, name string) ([]DNSRecord, error) {
	query := url.Values{"type": {"A"}, "name": {strings.TrimSpace(name)}, "per_page": {"100"}}
	var records []DNSRecord
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records"
	if err := client.do(ctx, http.MethodGet, path, query, nil, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// CreateARecord creates an A record.
func (client *HTTPClient) CreateARecord(ctx context.Context, zoneID string, input RecordInput) (*DNSRecord, error) {
	var record DNSRecord
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records"
	if err := client.do(ctx, http.MethodPost, path, nil, input, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateARecord replaces an A record.
func (client *HTTPClient) UpdateARecord(ctx context.Context, zoneID, recordID string, input RecordInput) (*DNSRecord, error) {
	var record DNSRecord
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(recordID)
	if err := client.do(ctx, http.MethodPut, path, nil, input, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// DeleteRecord deletes a DNS record.
func (client *HTTPClient) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	var result struct {
		ID string `json:"id"`
	}
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(recordID)
	return client.do(ctx, http.MethodDelete, path, nil, nil, &result)
}

func (client *HTTPClient) do(ctx context.Context, method, path string, query url.Values, body, result any) error {
	encodedBody, err := encodeRequestBody(body)
	if err != nil {
		return err
	}
	requestURL := buildRequestURL(client.baseURL, path, query)
	for attempt := 0; attempt < maxRequestAttempts; attempt++ {
		statusCode, retryHeader, responseBody, requestErr := client.send(ctx, method, requestURL, encodedBody)
		if requestErr != nil {
			return requestErr
		}
		if statusCode == http.StatusTooManyRequests && attempt < maxRequestAttempts-1 {
			if waitErr := waitForRetry(ctx, retryAfter(retryHeader)); waitErr != nil {
				return waitErr
			}
			continue
		}
		return decodeAPIResponse(statusCode, responseBody, result)
	}
	return errors.New("cloudflare API 请求超过重试次数")
}

func encodeRequestBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	encodedBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode Cloudflare request: %w", err)
	}
	return encodedBody, nil
}

func buildRequestURL(baseURL, path string, query url.Values) string {
	requestURL := baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	return requestURL
}

func (client *HTTPClient) send(ctx context.Context, method, requestURL string, body []byte) (int, string, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", nil, fmt.Errorf("create Cloudflare request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, "", nil, fmt.Errorf("cloudflare API 请求失败: %w", err)
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBodyBytes))
	closeErr := response.Body.Close()
	if readErr != nil {
		return 0, "", nil, fmt.Errorf("read Cloudflare response: %w", readErr)
	}
	if closeErr != nil {
		return 0, "", nil, fmt.Errorf("close Cloudflare response: %w", closeErr)
	}
	return response.StatusCode, response.Header.Get("Retry-After"), responseBody, nil
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func decodeAPIResponse(statusCode int, responseBody []byte, result any) error {
	var envelope apiEnvelope[json.RawMessage]
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return fmt.Errorf("decode Cloudflare response: %w", err)
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices || !envelope.Success {
		message := "cloudflare API 请求失败"
		if len(envelope.Errors) > 0 && strings.TrimSpace(envelope.Errors[0].Message) != "" {
			message = envelope.Errors[0].Message
		}
		return errors.New(message)
	}
	if result == nil || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode Cloudflare result: %w", err)
	}
	return nil
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return defaultRetryDelay
	}
	if seconds > maxRetryAfterSeconds {
		seconds = maxRetryAfterSeconds
	}
	return time.Duration(seconds) * time.Second
}
