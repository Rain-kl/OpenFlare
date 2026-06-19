// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UptimeKumaMonitor represents a monitor entry from Uptime Kuma.
type UptimeKumaMonitor struct {
	ID            int             `json:"id"`
	Name          string          `json:"name"`
	Url           string          `json:"url"`
	Type          string          `json:"type"`
	Interval      int             `json:"interval"`
	MaxRetries    int             `json:"maxretries"`
	RetryInterval int             `json:"retryInterval"`
	Timeout       int             `json:"timeout"`
	Tags          []UptimeKumaTag `json:"tags"`
}

// UptimeKumaTag represents a tag attached to a monitor.
type UptimeKumaTag struct {
	ID    int    `json:"tag_id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// UptimeKumaTagItem represents a tag returned by getTags.
type UptimeKumaTagItem struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// SocketIOClient is a minimal Engine.IO/Socket.IO polling client for Uptime Kuma.
type SocketIOClient struct {
	baseURL    string
	httpClient *http.Client
	sid        string
	ackMutex   sync.Mutex
	ackID      int
	ackChanMap map[int]chan string
	doneChan   chan struct{}
	closeOnce  sync.Once

	monitorListMutex sync.RWMutex
	monitorList      map[string]UptimeKumaMonitor
	monitorListChan  chan struct{}
	monitorListOnce  sync.Once

	ctx    context.Context
	cancel context.CancelFunc

	err error
}

// NewSocketIOClient creates a Socket.IO polling client for the given base URL.
func NewSocketIOClient(baseURL string) *SocketIOClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &SocketIOClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		ackChanMap:      make(map[int]chan string),
		doneChan:        make(chan struct{}),
		monitorListChan: make(chan struct{}),
		monitorList:     make(map[string]UptimeKumaMonitor),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Connect performs the Engine.IO handshake and starts the polling loop.
func (c *SocketIOClient) Connect() error {
	slog.Debug("Uptime Kuma client starting handshake", "baseURL", c.baseURL)
	u := fmt.Sprintf("%s/socket.io/?EIO=4&transport=polling", c.baseURL)
	reqHandshake, err := http.NewRequestWithContext(c.ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("create handshake request failed: %w", err)
	}
	resp, err := c.httpClient.Do(reqHandshake)
	if err != nil {
		slog.Error("Uptime Kuma handshake connection failed", "url", u, "error", err)
		return fmt.Errorf("handshake request failed: %w", err)
	}
	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read Uptime Kuma handshake response body", "error", err)
		return fmt.Errorf("read handshake body failed: %w", err)
	}

	bodyStr := string(bs)
	slog.Debug("Received handshake response from Uptime Kuma", "body", bodyStr)
	if len(bodyStr) == 0 || bodyStr[0] != '0' {
		return fmt.Errorf("invalid handshake response format: %s", bodyStr)
	}

	var hs struct {
		Sid string `json:"sid"`
	}
	if err := json.Unmarshal([]byte(bodyStr[1:]), &hs); err != nil {
		return fmt.Errorf("unmarshal handshake sid failed: %w", err)
	}
	c.sid = hs.Sid
	slog.Debug("Uptime Kuma handshake success", "sid", c.sid)

	slog.Debug("Sending namespace connect request to Uptime Kuma", "sid", c.sid)
	connectURL := fmt.Sprintf("%s/socket.io/?EIO=4&transport=polling&sid=%s", c.baseURL, c.sid)
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, connectURL, strings.NewReader("40"))
	if err != nil {
		return fmt.Errorf("create connect request failed: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	respConnect, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Uptime Kuma namespace connect request failed", "sid", c.sid, "error", err)
		return fmt.Errorf("namespace connect failed: %w", err)
	}
	respConnect.Body.Close()
	slog.Debug("Namespace connected successfully to Uptime Kuma", "sid", c.sid)

	go c.pollLoop()

	return nil
}

func (c *SocketIOClient) pollLoop() {
	slog.Debug("Uptime Kuma polling loop started", "sid", c.sid)
	defer c.Close()
	for {
		select {
		case <-c.doneChan:
			slog.Debug("Uptime Kuma polling loop stopped (doneChan closed)", "sid", c.sid)
			return
		default:
		}

		u := fmt.Sprintf("%s/socket.io/?EIO=4&transport=polling&sid=%s", c.baseURL, c.sid)
		reqPoll, err := http.NewRequestWithContext(c.ctx, http.MethodGet, u, nil)
		if err != nil {
			slog.Error("Failed to create Uptime Kuma polling request", "sid", c.sid, "error", err)
			c.err = err
			return
		}
		resp, err := c.httpClient.Do(reqPoll)
		if err != nil {
			slog.Error("Uptime Kuma polling request failed", "sid", c.sid, "error", err)
			c.err = err
			return
		}

		bs, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			slog.Error("Failed to read Uptime Kuma polling body", "sid", c.sid, "error", err)
			c.err = err
			return
		}

		bodyStr := string(bs)
		if len(bodyStr) == 0 {
			continue
		}

		slog.Debug("Received polling payload from Uptime Kuma", "length", len(bodyStr))
		packets := strings.Split(bodyStr, "\x1e")
		for _, pkt := range packets {
			if len(pkt) == 0 {
				continue
			}
			engineIOType := pkt[0]
			payload := pkt[1:]

			slog.Debug("Parsing engine.io packet", "type", string(engineIOType), "payload_len", len(payload))
			switch engineIOType {
			case '2':
				slog.Debug("Received engine.io ping, responding with pong", "sid", c.sid)
				c.sendPong()
			case '4':
				if len(payload) == 0 {
					continue
				}
				socketIOType := payload[0]
				socketIOPayload := payload[1:]

				slog.Debug("Parsing socket.io packet", "type", string(socketIOType), "payload", socketIOPayload)
				switch socketIOType {
				case '2':
					c.handleEvent(socketIOPayload)
				case '3':
					c.handleAck(socketIOPayload)
				}
			}
		}
	}
}

func (c *SocketIOClient) sendPong() {
	u := fmt.Sprintf("%s/socket.io/?EIO=4&transport=polling&sid=%s", c.baseURL, c.sid)
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, u, strings.NewReader("3"))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	resp, err := c.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func (c *SocketIOClient) handleEvent(payload string) {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &arr); err != nil || len(arr) < 2 {
		return
	}
	var eventName string
	if err := json.Unmarshal(arr[0], &eventName); err != nil {
		return
	}
	if eventName == "monitorList" {
		var list map[string]UptimeKumaMonitor
		if err := json.Unmarshal(arr[1], &list); err == nil {
			c.monitorListMutex.Lock()
			c.monitorList = list
			c.monitorListMutex.Unlock()
			c.monitorListOnce.Do(func() {
				close(c.monitorListChan)
			})
		}
	}
}

func (c *SocketIOClient) handleAck(payload string) {
	idx := strings.IndexByte(payload, '[')
	if idx == -1 {
		return
	}
	ackIDStr := payload[:idx]
	ackID, err := strconv.Atoi(ackIDStr)
	if err != nil {
		return
	}
	c.ackMutex.Lock()
	ch, ok := c.ackChanMap[ackID]
	if ok {
		delete(c.ackChanMap, ackID)
		c.ackMutex.Unlock()
		select {
		case ch <- payload[idx:]:
		default:
		}
	} else {
		c.ackMutex.Unlock()
	}
}

// Emit sends a Socket.IO event and waits for the corresponding ack.
func (c *SocketIOClient) Emit(event string, args ...any) (string, error) {
	c.ackMutex.Lock()
	id := c.ackID
	c.ackID++
	ch := make(chan string, 1)
	c.ackChanMap[id] = ch
	c.ackMutex.Unlock()

	payloadArr := []any{event}
	payloadArr = append(payloadArr, args...)
	bs, err := json.Marshal(payloadArr)
	if err != nil {
		c.ackMutex.Lock()
		delete(c.ackChanMap, id)
		c.ackMutex.Unlock()
		slog.Error("Failed to marshal event payload", "event", event, "error", err)
		return "", err
	}

	body := fmt.Sprintf("42%d%s", id, string(bs))
	slog.Debug("Emitting Socket.IO event", "event", event, "ackID", id, "payload", string(bs))

	u := fmt.Sprintf("%s/socket.io/?EIO=4&transport=polling&sid=%s", c.baseURL, c.sid)
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, u, strings.NewReader(body))
	if err != nil {
		c.ackMutex.Lock()
		delete(c.ackChanMap, id)
		c.ackMutex.Unlock()
		return "", err
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.ackMutex.Lock()
		delete(c.ackChanMap, id)
		c.ackMutex.Unlock()
		slog.Error("Failed to send Emit request", "event", event, "ackID", id, "error", err)
		return "", err
	}
	resp.Body.Close()

	select {
	case result := <-ch:
		slog.Debug("Received Ack for event", "event", event, "ackID", id, "response", result)
		return result, nil
	case <-time.After(10 * time.Second):
		c.ackMutex.Lock()
		delete(c.ackChanMap, id)
		c.ackMutex.Unlock()
		slog.Error("Timeout waiting for event Ack", "event", event, "ackID", id)
		return "", fmt.Errorf("timeout waiting for ack for event: %s", event)
	case <-c.doneChan:
		c.ackMutex.Lock()
		delete(c.ackChanMap, id)
		c.ackMutex.Unlock()
		slog.Error("Client closed while waiting for event Ack", "event", event, "ackID", id)
		return "", fmt.Errorf("client closed while waiting for event ack: %s", event)
	}
}

// Close shuts down the polling loop.
func (c *SocketIOClient) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.doneChan)
	})
}

// GetMonitorListChan returns a channel closed when the first monitorList event arrives.
func (c *SocketIOClient) GetMonitorListChan() <-chan struct{} {
	return c.monitorListChan
}

// GetMonitorList returns a copy of the current monitor list.
func (c *SocketIOClient) GetMonitorList() map[string]UptimeKumaMonitor {
	c.monitorListMutex.RLock()
	defer c.monitorListMutex.RUnlock()

	m := make(map[string]UptimeKumaMonitor, len(c.monitorList))
	for k, v := range c.monitorList {
		m[k] = v
	}
	return m
}

// ParseAckResponse unmarshals an ack payload and validates the ok status when present.
func ParseAckResponse(response string, target any) error {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(response), &arr); err != nil || len(arr) == 0 {
		return fmt.Errorf("invalid ack response format: %s", response)
	}

	var status struct {
		Ok  bool   `json:"ok"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(arr[0], &status); err == nil {
		if !status.Ok {
			errMsg := status.Msg
			if errMsg == "" {
				errMsg = "unknown error from Uptime Kuma"
			}
			return fmt.Errorf("Uptime Kuma error response: %s", errMsg)
		}
	}

	if target != nil {
		return json.Unmarshal(arr[0], target)
	}
	return nil
}
