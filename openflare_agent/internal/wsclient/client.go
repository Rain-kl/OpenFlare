package wsclient

import (
	"context"
	"time"

	"openflare-agent/internal/protocol"
	shared "openflare/utils/wsclient"
)

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
			HeaderKey: "X-Agent-Token",
			WSPath:    "/api/agent/ws",
		}),
	}
}

func (c *Client) SetToken(token string) {
	c.sharedClient.SetToken(token)
}

func (c *Client) URL() string {
	return c.sharedClient.URL()
}

func (c *Client) Connect(ctx context.Context) (protocol.WebSocketConnection, error) {
	conn, err := c.sharedClient.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &Connection{sharedConn: conn}, nil
}

func (conn *Connection) URL() string {
	if conn == nil || conn.sharedConn == nil {
		return ""
	}
	return conn.sharedConn.URL
}

func (conn *Connection) SendStatus(payload protocol.NodePayload) error {
	return conn.sharedConn.SendMessage(protocol.WSMessageTypeStatus, payload)
}

func (conn *Connection) SendPong() error {
	return conn.sharedConn.SendMessage(protocol.WSMessageTypePong, nil)
}

func (conn *Connection) Receive() (protocol.WSMessage, error) {
	var message protocol.WSMessage
	if err := conn.sharedConn.Receive(&message); err != nil {
		return message, err
	}
	return message, nil
}

func (conn *Connection) Close() error {
	if conn == nil || conn.sharedConn == nil {
		return nil
	}
	return conn.sharedConn.Close()
}
