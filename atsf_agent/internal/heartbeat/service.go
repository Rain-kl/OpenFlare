package heartbeat

import (
	"context"

	"atsflare-agent/internal/protocol"
)

type Client interface {
	RegisterNode(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error)
	Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error)
	SetToken(token string)
}

type Service struct {
	client Client
}

func New(client Client) *Service {
	return &Service{client: client}
}

func (s *Service) Register(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error) {
	return s.client.RegisterNode(ctx, payload)
}

func (s *Service) Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error) {
	return s.client.Heartbeat(ctx, payload)
}

func (s *Service) SetToken(token string) {
	s.client.SetToken(token)
}
