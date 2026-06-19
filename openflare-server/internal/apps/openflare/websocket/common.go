// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	messageTypePing   = "ping"
	messageTypePong   = "pong"
	messageTypeNotify = "notify"
)

// Message is a JSON-framed websocket payload.
type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}
