package wsclient

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"openflare-agent/internal/protocol"
)

type Client struct {
	baseURL string
	token   string
	timeout time.Duration
}

type Connection struct {
	conn        *websocket.Conn
	url         string
	readTimeout time.Duration
}

func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		timeout: timeout,
	}
}

func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
	slog.Debug("agent ws client token updated")
}

func (c *Client) URL() string {
	wsURL, err := buildWebsocketURL(c.baseURL)
	if err != nil {
		return ""
	}
	return wsURL
}

func (c *Client) Connect(ctx context.Context) (protocol.WebSocketConnection, error) {
	wsURL, err := buildWebsocketURL(c.baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.token) == "" {
		return nil, errors.New("agent ws token is empty")
	}
	origin := strings.TrimSpace(c.baseURL)
	if origin == "" {
		origin = "http://localhost"
	}
	config, err := websocket.NewConfig(wsURL, origin)
	if err != nil {
		return nil, err
	}
	config.Header = http.Header{}
	config.Header.Set("X-Agent-Token", c.token)
	if c.timeout > 0 {
		config.Dialer = &net.Dialer{Timeout: c.timeout}
	}
	slog.Debug("agent ws dialing server", "url", wsURL)
	conn, err := config.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	slog.Debug("agent ws dial succeeded", "url", wsURL)
	return &Connection{conn: conn, url: wsURL, readTimeout: websocketReadTimeout(c.timeout)}, nil
}

func buildWebsocketURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("server_url scheme must be http, https, ws, or wss")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/agent/ws"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (conn *Connection) URL() string {
	if conn == nil {
		return ""
	}
	return conn.url
}

func (conn *Connection) SendStatus(payload protocol.NodePayload) error {
	if conn == nil || conn.conn == nil {
		return errors.New("agent ws connection is nil")
	}
	slog.Debug("agent ws sending status",
		"node_id", payload.NodeID,
		"current_version", payload.CurrentVersion,
		"openresty_status", payload.OpenrestyStatus,
	)
	return websocket.JSON.Send(conn.conn, protocol.WSOutboundMessage{
		Type:    protocol.WSMessageTypeStatus,
		Payload: payload,
	})
}

func (conn *Connection) SendPong() error {
	if conn == nil || conn.conn == nil {
		return errors.New("agent ws connection is nil")
	}
	slog.Debug("agent ws sending pong")
	return websocket.JSON.Send(conn.conn, protocol.WSOutboundMessage{
		Type: protocol.WSMessageTypePong,
	})
}

func (conn *Connection) Receive() (protocol.WSMessage, error) {
	var message protocol.WSMessage
	if conn == nil || conn.conn == nil {
		return message, errors.New("agent ws connection is nil")
	}
	if conn.readTimeout > 0 {
		_ = conn.conn.SetReadDeadline(time.Now().Add(conn.readTimeout))
	}
	err := websocket.JSON.Receive(conn.conn, &message)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			slog.Debug("agent ws receive timeout waiting for server message", "timeout", conn.readTimeout)
		}
		return message, err
	}
	slog.Debug("agent ws received message", "type", message.Type)
	return message, nil
}

func websocketReadTimeout(requestTimeout time.Duration) time.Duration {
	timeout := requestTimeout * 6
	if timeout < 75*time.Second {
		return 75 * time.Second
	}
	return timeout
}

func (conn *Connection) Close() error {
	if conn == nil || conn.conn == nil {
		return nil
	}
	return conn.conn.Close()
}
