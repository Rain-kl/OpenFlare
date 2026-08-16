// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package heartbeat

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

// RemoteClient is the interface that abstracts the remote API calls performed by Service.
type RemoteClient interface {
	RegisterNode(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error)
	Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error)
	SetToken(token string)
}

// API abstracts registration and heartbeat operations used by Cycle.
type API interface {
	Register(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error)
	Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error)
	SetToken(token string)
}

// Service wraps a RemoteClient to expose agent registration and heartbeat operations.
type Service struct {
	client RemoteClient
}

// New creates a new Service backed by the given RemoteClient.
func New(client RemoteClient) *Service {
	return &Service{client: client}
}

// Register sends a node registration request to the server.
func (s *Service) Register(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error) {
	return s.client.RegisterNode(ctx, payload)
}

// Heartbeat sends a heartbeat to the server using the service client.
func (s *Service) Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error) {
	return s.client.Heartbeat(ctx, payload)
}

// SetToken sets the authentication token for the service client.
func (s *Service) SetToken(token string) {
	s.client.SetToken(token)
}
