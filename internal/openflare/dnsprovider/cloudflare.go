// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsprovider

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
	cloudflareActive     = "active"
	defaultAPIBaseURL    = "https://api.cloudflare.com/client/v4"
	defaultHTTPTimeout   = 20 * time.Second
	maxRequestAttempts   = 3
	maxResponseBodyBytes = 1 << 20
	defaultRetryDelay    = 200 * time.Millisecond
	maxRetryAfterSeconds = 2
)

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
	if result.Status != cloudflareActive {
		return errors.New("cloudflare API token is inactive")
	}
	return nil
}

// FindZone returns the exact Cloudflare zone name.
func (client *HTTPClient) FindZone(ctx context.Context, name string) (*Zone, error) {
	query := url.Values{"name": {strings.TrimSpace(name)}, "status": {cloudflareActive}, "per_page": {"5"}}
	var zones []Zone
	if err := client.do(ctx, http.MethodGet, "/zones", query, nil, &zones); err != nil {
		return nil, err
	}
	if len(zones) != 1 {
		return nil, fmt.Errorf("no unique Cloudflare zone found for %s", name)
	}
	return &zones[0], nil
}

// ListZones discovers active zones visible to the token.
func (client *HTTPClient) ListZones(ctx context.Context) ([]Zone, error) {
	return listCloudflarePages[Zone](ctx, client, "/zones", url.Values{"status": {cloudflareActive}})
}

func listCloudflarePages[T any](ctx context.Context, client *HTTPClient, path string, query url.Values) ([]T, error) {
	const pageSize = 50
	var result []T
	query.Set("per_page", strconv.Itoa(pageSize))
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		var items []T
		if err := client.do(ctx, http.MethodGet, path, query, nil, &items); err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(items) < pageSize {
			return result, nil
		}
	}
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

// ListRecords lists exact-name DNS records of a type.
func (client *HTTPClient) ListRecords(ctx context.Context, zoneID, name, recordType string) ([]DNSRecord, error) {
	query := url.Values{"name": {strings.TrimSpace(name)}}
	if recordType != "" {
		query.Set("type", recordType)
	}
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records"
	return listCloudflarePages[DNSRecord](ctx, client, path, query)
}

// CreateRecord creates a DNS record.
func (client *HTTPClient) CreateRecord(ctx context.Context, zoneID string, input RecordInput) (*DNSRecord, error) {
	var record DNSRecord
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records"
	if err := client.do(ctx, http.MethodPost, path, nil, input, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateRecord replaces a DNS record.
func (client *HTTPClient) UpdateRecord(ctx context.Context, zoneID, recordID string, input RecordInput) (*DNSRecord, error) {
	var record DNSRecord
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(recordID)
	if err := client.do(ctx, http.MethodPut, path, nil, input, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// ListARecords lists exact-name A records.
func (client *HTTPClient) ListARecords(ctx context.Context, zoneID, name string) ([]DNSRecord, error) {
	return client.ListRecords(ctx, zoneID, name, "A")
}

// CreateARecord creates an A record.
func (client *HTTPClient) CreateARecord(ctx context.Context, zoneID string, input RecordInput) (*DNSRecord, error) {
	return client.CreateRecord(ctx, zoneID, input)
}

// UpdateARecord replaces an A record.
func (client *HTTPClient) UpdateARecord(ctx context.Context, zoneID, recordID string, input RecordInput) (*DNSRecord, error) {
	return client.UpdateRecord(ctx, zoneID, recordID, input)
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
	for attempt := range maxRequestAttempts {
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
	return errors.New("cloudflare API retry limit exceeded")
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
		return 0, "", nil, fmt.Errorf("cloudflare API request failed: %w", err)
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
		message := "Cloudflare API request failed"
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
