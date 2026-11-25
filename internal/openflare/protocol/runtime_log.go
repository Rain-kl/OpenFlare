// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

// Runtime log WebSocket messages share the existing authenticated agent connection.
const (
	WSMessageTypeRuntimeLogSubscribe   = "runtime_log_subscribe"
	WSMessageTypeRuntimeLogUnsubscribe = "runtime_log_unsubscribe"
	WSMessageTypeRuntimeLogFrame       = "runtime_log_frame"
)

// RuntimeLogCommand identifies a live subscription on an agent WebSocket.
type RuntimeLogCommand struct {
	StreamID string `json:"stream_id"`
}

// RuntimeLogFrame carries a readiness acknowledgment or a bounded batch of runtime logs.
type RuntimeLogFrame struct {
	StreamID string   `json:"stream_id"`
	Ready    bool     `json:"ready,omitempty"`
	Lines    []string `json:"lines,omitempty"`
	Dropped  uint64   `json:"dropped,omitempty"`
}
