// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"encoding/json"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	runtimeLogOpenChannel  = "edge:runtime_logs:open"
	runtimeLogTopicPrefix  = "edge:runtime_logs:view:"
	runtimeLogRelayTimeout = 3 * RuntimeLogKeepAlive
)

type runtimeLogOpenRequest struct {
	NodeID string `json:"node_id"`
	ViewID string `json:"view_id"`
}

type runtimeLogPublication struct {
	Frame  *protocol.RuntimeLogFrame `json:"frame,omitempty"`
	Closed bool                      `json:"closed,omitempty"`
}

func openRemoteRuntimeLogs(ctx context.Context, nodeID string) (*RuntimeLogStream, error) {
	client := db.Redis
	if client == nil {
		return nil, ErrRuntimeLogsUnavailable
	}
	viewID := uuid.NewString()
	readyCtx, stopWaiting := context.WithTimeout(ctx, RuntimeLogHandshakeTimeout)
	defer stopWaiting()
	subscription := client.Subscribe(ctx, db.PrefixedKey(runtimeLogTopicPrefix+viewID))
	if _, err := subscription.Receive(readyCtx); err != nil {
		_ = subscription.Close()
		return nil, err
	}
	request, err := json.Marshal(runtimeLogOpenRequest{NodeID: nodeID, ViewID: viewID})
	if err == nil {
		var receivers int64
		receivers, err = client.Publish(readyCtx, db.PrefixedKey(runtimeLogOpenChannel), request).Result()
		if err == nil && receivers == 0 {
			err = ErrRuntimeLogsUnavailable
		}
	}
	if err != nil {
		_ = subscription.Close()
		return nil, err
	}

	listenCtx, cancel := context.WithCancel(ctx)
	frames := make(chan protocol.RuntimeLogFrame, runtimeLogQueueSize)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(frames)
		defer func() { _ = subscription.Close() }()
		consumeRuntimeLogPublications(listenCtx, subscription.Channel(redis.WithChannelSize(runtimeLogQueueSize), redis.WithChannelSendTimeout(time.Second)), frames)
	}()
	return &RuntimeLogStream{Frames: frames, Done: done, close: cancel}, nil
}

func consumeRuntimeLogPublications(ctx context.Context, messages <-chan *redis.Message, frames chan<- protocol.RuntimeLogFrame) {
	idle := time.NewTimer(runtimeLogRelayTimeout)
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-idle.C:
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			var publication runtimeLogPublication
			if json.Unmarshal([]byte(message.Payload), &publication) != nil || publication.Closed {
				return
			}
			idle.Reset(runtimeLogRelayTimeout)
			if publication.Frame != nil {
				select {
				case frames <- *publication.Frame:
				default:
					return
				}
			}
		}
	}
}

// StartRuntimeLogRelay lets a controller forward logs when it owns the target agent WebSocket.
func StartRuntimeLogRelay(ctx context.Context) {
	client := db.Redis
	if client == nil {
		return
	}
	relayCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	subscription := client.Subscribe(relayCtx, db.PrefixedKey(runtimeLogOpenChannel))
	lifecycle.OnShutdown("edge runtime log relay", func(context.Context) error {
		cancel()
		return subscription.Close()
	})
	slots := make(chan struct{}, maxRuntimeLogStreams)
	go func() {
		for message := range subscription.Channel() {
			var request runtimeLogOpenRequest
			if json.Unmarshal([]byte(message.Payload), &request) != nil || uuid.Validate(request.ViewID) != nil {
				continue
			}
			agent := localRuntimeLogAgent(request.NodeID)
			if agent == nil {
				continue
			}
			select {
			case slots <- struct{}{}:
			default:
				continue
			}
			go func() {
				defer func() { <-slots }()
				relayRuntimeLogs(relayCtx, client, agent, db.PrefixedKey(runtimeLogTopicPrefix+request.ViewID))
			}()
		}
	}()
}

func relayRuntimeLogs(ctx context.Context, client redis.UniversalClient, agent *agentClient, topic string) {
	stream, err := agent.subscribeRuntimeLogs()
	if err != nil {
		return
	}
	defer stream.Close()

	readyCtx, cancel := context.WithTimeout(ctx, RuntimeLogHandshakeTimeout)
	readyErr := WaitRuntimeLogReady(readyCtx, stream)
	cancel()
	if readyErr != nil || !publishRuntimeLogs(ctx, client, topic, runtimeLogPublication{Frame: &protocol.RuntimeLogFrame{Ready: true}}) {
		return
	}
	defer publishRuntimeLogs(ctx, client, topic, runtimeLogPublication{Closed: true})

	ticker := time.NewTicker(RuntimeLogKeepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stream.Done:
			return
		case frame, ok := <-stream.Frames:
			if !ok || !publishRuntimeLogs(ctx, client, topic, runtimeLogPublication{Frame: &frame}) {
				return
			}
		case <-ticker.C:
			if !publishRuntimeLogs(ctx, client, topic, runtimeLogPublication{}) {
				return
			}
			stream.Renew()
		}
	}
}

// These bytes are encoded for Redis but redacted when Redis tracing formats command arguments.
type runtimeLogPayload []byte

func (p runtimeLogPayload) MarshalBinary() ([]byte, error) { return p, nil }
func (runtimeLogPayload) String() string                   { return "<runtime log payload>" }

func publishRuntimeLogs(ctx context.Context, client redis.UniversalClient, topic string, message runtimeLogPublication) bool {
	data, err := json.Marshal(message)
	if err != nil {
		return false
	}
	count, err := client.Publish(ctx, topic, runtimeLogPayload(data)).Result()
	return err == nil && count > 0
}

// WaitRuntimeLogReady waits for the target Edge to acknowledge a log subscription.
func WaitRuntimeLogReady(ctx context.Context, stream *RuntimeLogStream) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stream.Done:
		return ErrRuntimeLogsUnavailable
	case frame, ok := <-stream.Frames:
		if ok && frame.Ready {
			return nil
		}
		return ErrRuntimeLogsUnavailable
	}
}
