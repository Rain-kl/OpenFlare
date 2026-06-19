package wsclient

import (
	"context"
	"encoding/json"
	"time"

	service "github.com/Rain-kl/Wavelet/pkg/protocol"
	shared "github.com/Rain-kl/Wavelet/pkg/wsclient"
)

type WSMessage = shared.WSMessage
type MessageHandler = shared.MessageHandler

type Client struct {
	sharedClient *shared.Client
}

type Connection struct {
	sharedConn *shared.Connection
}

func New(baseURL string, token string, timeout time.Duration) *Client {
	return &Client{
		sharedClient: shared.New(shared.Config{
			BaseURL:   baseURL,
			Token:     token,
			Timeout:   timeout,
			HeaderKey: "X-Tunnel-Token",
			WSPath:    "/api/v1/tunnel/ws",
		}),
	}
}

func (c *Client) SetToken(token string) {
	c.sharedClient.SetToken(token)
}

func (c *Client) Connect(ctx context.Context) (*Connection, error) {
	conn, err := c.sharedClient.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &Connection{sharedConn: conn}, nil
}

func (conn *Connection) SendPing() error {
	return conn.sharedConn.SendMessage("ping", nil)
}

func (conn *Connection) SendPong() error {
	return conn.sharedConn.SendMessage("pong", nil)
}

func (conn *Connection) Receive() (service.WSMessage, error) {
	var raw struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := conn.sharedConn.Receive(&raw); err != nil {
		return service.WSMessage{}, err
	}
	return service.WSMessage{
		Type:    raw.Type,
		Payload: raw.Payload,
	}, nil
}

func (conn *Connection) RunReceiveLoop(ctx context.Context, handler shared.MessageHandler) error {
	return conn.sharedConn.RunReceiveLoop(ctx, handler)
}

func (conn *Connection) Close() error {
	return conn.sharedConn.Close()
}
