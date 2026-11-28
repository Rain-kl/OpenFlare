// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/extra/rediscmd/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRuntimeLogRelayUsesEphemeralRedactedPublications(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	previous := db.Redis
	db.Redis = client
	t.Cleanup(func() {
		db.Redis = previous
		_ = client.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requests := client.Subscribe(ctx, db.PrefixedKey(runtimeLogOpenChannel))
	defer func() { _ = requests.Close() }()
	_, err := requests.Receive(ctx)
	require.NoError(t, err)

	view, err := openRemoteRuntimeLogs(ctx, "edge-test")
	require.NoError(t, err)
	defer view.Close()

	event, err := requests.ReceiveMessage(ctx)
	require.NoError(t, err)
	var request runtimeLogOpenRequest
	require.NoError(t, json.Unmarshal([]byte(event.Payload), &request))
	require.Equal(t, "edge-test", request.NodeID)
	require.NotEmpty(t, request.ViewID)

	topic := db.PrefixedKey(runtimeLogTopicPrefix + request.ViewID)
	agent := &agentClient{wsClientCore: wsClientCore{send: make(chan Message, 2), done: make(chan struct{})}}
	relayed := make(chan struct{})
	go func() {
		defer close(relayed)
		relayRuntimeLogs(ctx, client, agent, topic)
	}()

	command := receiveRuntimeLogTestEvent(t, ctx, agent.send)
	require.Equal(t, protocol.WSMessageTypeRuntimeLogSubscribe, command.Type)
	subscription := command.Payload.(protocol.RuntimeLogCommand)
	agent.dispatchRuntimeLogFrame(protocol.RuntimeLogFrame{StreamID: subscription.StreamID, Ready: true})
	require.NoError(t, WaitRuntimeLogReady(ctx, view))

	const privateLine = "example private runtime log"
	payload := runtimeLogPayload(privateLine)
	require.NotContains(t, rediscmd.CmdString(redis.NewIntCmd(ctx, "publish", topic, payload)), privateLine)
	agent.dispatchRuntimeLogFrame(protocol.RuntimeLogFrame{StreamID: subscription.StreamID, Lines: []string{privateLine}})
	frame := receiveRuntimeLogTestEvent(t, ctx, view.Frames)
	require.Equal(t, []string{privateLine}, frame.Lines)
	require.Empty(t, redisServer.Keys())

	view.Close()
	receiveRuntimeLogTestEvent(t, ctx, view.Done)
	require.Eventually(t, func() bool {
		return redisServer.PubSubNumSub(topic)[topic] == 0
	}, time.Second, 10*time.Millisecond, "runtime log viewer should unsubscribe before relay cleanup")
	// A new publication detects that the viewing replica has unsubscribed.
	agent.dispatchRuntimeLogFrame(protocol.RuntimeLogFrame{StreamID: subscription.StreamID, Lines: []string{"after closing"}})
	receiveRuntimeLogTestEvent(t, ctx, relayed)
	require.Equal(t, protocol.WSMessageTypeRuntimeLogUnsubscribe, receiveRuntimeLogTestEvent(t, ctx, agent.send).Type)
}

func receiveRuntimeLogTestEvent[T any](t *testing.T, ctx context.Context, events <-chan T) T {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-ctx.Done():
		t.Fatal("timed out waiting for runtime log stream event")
		var zero T
		return zero
	}
}
