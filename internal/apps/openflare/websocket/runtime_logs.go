// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/google/uuid"
)

const (
	// RuntimeLogHandshakeTimeout bounds the wait for the node to acknowledge the subscription.
	RuntimeLogHandshakeTimeout = 5 * time.Second
	// RuntimeLogKeepAlive keeps the SSE connection and the Edge lease active.
	RuntimeLogKeepAlive    = 10 * time.Second
	runtimeLogQueueSize    = 8
	maxRuntimeLogViewers   = 8
	maxRuntimeLogStreams   = 64
	maxRuntimeLogLines     = 8
	maxRuntimeLogLineBytes = 4098 // Edge caps formatted bytes at 4096 before UTF-8 replacement.
	maxAgentMessageBytes   = 256 << 10
)

// ErrRuntimeLogsUnavailable indicates the control connection cannot accept a subscription.
var ErrRuntimeLogsUnavailable = errors.New("node runtime log stream unavailable")

var (
	runtimeLogSlots                           = make(chan struct{}, maxRuntimeLogStreams)
	runtimeLogShutdown, stopRuntimeLogStreams = context.WithCancel(context.Background())
)

// StopRuntimeLogStreams closes active browser streams during HTTP shutdown.
func StopRuntimeLogStreams() {
	stopRuntimeLogStreams()
}

// RuntimeLogStream holds a transient subscription to an agent or its controller replica.
type RuntimeLogStream struct {
	Frames <-chan protocol.RuntimeLogFrame
	Done   <-chan struct{}
	close  func()
	renew  func()
}

// Close releases the subscription and its buffers.
func (s *RuntimeLogStream) Close() {
	s.close()
}

// Renew extends the lease of a local Edge subscription.
func (s *RuntimeLogStream) Renew() {
	if s.renew != nil {
		s.renew()
	}
}

type agentRuntimeLogs struct {
	mu      sync.Mutex
	id      string
	ready   bool
	viewers map[chan protocol.RuntimeLogFrame]struct{}
}

// OpenRuntimeLogStream subscribes to a live agent over its authenticated WebSocket.
func OpenRuntimeLogStream(ctx context.Context, nodeID string) (*RuntimeLogStream, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-runtimeLogShutdown.Done():
		return nil, ErrRuntimeLogsUnavailable
	case runtimeLogSlots <- struct{}{}:
	default:
		return nil, ErrRuntimeLogsUnavailable
	}

	streamCtx, cancel := context.WithCancel(ctx)
	client := localRuntimeLogAgent(nodeID)
	var stream *RuntimeLogStream
	var err error
	if client != nil {
		stream, err = client.subscribeRuntimeLogs()
	} else {
		stream, err = openRemoteRuntimeLogs(streamCtx, nodeID)
	}
	if err != nil {
		cancel()
		<-runtimeLogSlots
		return nil, err
	}

	closeSubscription := stream.close
	var once sync.Once
	stream.close = func() {
		once.Do(func() {
			cancel()
			closeSubscription()
			<-runtimeLogSlots
		})
	}
	// Terminate both local and relayed streams during HTTP shutdown.
	sourceDone := stream.Done
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-sourceDone:
		case <-streamCtx.Done():
		case <-runtimeLogShutdown.Done():
		}
	}()
	stream.Done = done
	return stream, nil
}

func localRuntimeLogAgent(nodeID string) *agentClient {
	defaultAgentHub.mu.RLock()
	defer defaultAgentHub.mu.RUnlock()
	return defaultAgentHub.clients[nodeID]
}

func (c *agentClient) subscribeRuntimeLogs() (*RuntimeLogStream, error) {
	logs := &c.runtimeLogs
	logs.mu.Lock()
	defer logs.mu.Unlock()
	select {
	case <-c.done:
		return nil, ErrRuntimeLogsUnavailable
	default:
	}
	if len(logs.viewers) >= maxRuntimeLogViewers {
		return nil, ErrRuntimeLogsUnavailable
	}
	if logs.id == "" {
		id := uuid.NewString()
		if !c.sendRuntimeLogCommand(protocol.WSMessageTypeRuntimeLogSubscribe, id) {
			return nil, ErrRuntimeLogsUnavailable
		}
		logs.id = id
		logs.viewers = make(map[chan protocol.RuntimeLogFrame]struct{})
	}
	id := logs.id
	frames := make(chan protocol.RuntimeLogFrame, runtimeLogQueueSize)
	logs.viewers[frames] = struct{}{}
	if logs.ready {
		frames <- protocol.RuntimeLogFrame{Ready: true}
	}
	return &RuntimeLogStream{
		Frames: frames,
		Done:   c.done,
		close:  func() { c.removeRuntimeLogViewer(frames) },
		renew: func() {
			logs.mu.Lock()
			defer logs.mu.Unlock()
			if logs.id == id {
				c.sendRuntimeLogCommand(protocol.WSMessageTypeRuntimeLogSubscribe, id)
			}
		},
	}, nil
}

func (c *agentClient) sendRuntimeLogCommand(kind, id string) bool {
	return c.enqueue(Message{Type: kind, Payload: protocol.RuntimeLogCommand{StreamID: id}})
}

func (c *agentClient) removeRuntimeLogViewer(frames chan protocol.RuntimeLogFrame) {
	logs := &c.runtimeLogs
	logs.mu.Lock()
	defer logs.mu.Unlock()
	if _, exists := logs.viewers[frames]; exists {
		delete(logs.viewers, frames)
		close(frames)
		c.stopUnusedRuntimeLogs()
	}
}

func (c *agentClient) stopUnusedRuntimeLogs() {
	logs := &c.runtimeLogs
	if len(logs.viewers) == 0 && logs.id != "" {
		// If the send queue is saturated, Edge drops the subscription when its lease expires.
		c.sendRuntimeLogCommand(protocol.WSMessageTypeRuntimeLogUnsubscribe, logs.id)
		logs.id, logs.ready = "", false
	}
}

func (c *agentClient) dispatchRuntimeLogFrame(frame protocol.RuntimeLogFrame) {
	if len(frame.Lines) > maxRuntimeLogLines {
		return
	}
	for _, line := range frame.Lines {
		if len(line) > maxRuntimeLogLineBytes {
			return
		}
	}
	logs := &c.runtimeLogs
	logs.mu.Lock()
	defer logs.mu.Unlock()
	if logs.id == "" || logs.id != frame.StreamID {
		return
	}
	if frame.Ready {
		if logs.ready {
			return
		}
		logs.ready = true
	}
	if !logs.ready {
		return
	}
	frame.StreamID = ""
	for viewer := range logs.viewers {
		select {
		case viewer <- frame:
		default:
			// A slow viewer cannot stall the agent control connection.
			delete(logs.viewers, viewer)
			close(viewer)
		}
	}
	c.stopUnusedRuntimeLogs()
}
