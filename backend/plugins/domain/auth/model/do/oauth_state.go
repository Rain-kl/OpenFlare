// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package do provides domain data objects for the auth plugin.
package do

import "encoding/json"

// OAuthStatePayload represents the cached state verification payload for OAuth flow.
type OAuthStatePayload struct {
	SourceName  string `json:"source_name"`
	Purpose     string `json:"purpose"`
	UserID      uint64 `json:"user_id,omitempty"`
	SessionHash string `json:"session_hash"`
}

// Encode converts OAuthStatePayload to a JSON string.
func (p OAuthStatePayload) Encode() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DecodeOAuthStatePayload parses a JSON string into OAuthStatePayload.
func DecodeOAuthStatePayload(value string) (OAuthStatePayload, error) {
	var payload OAuthStatePayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return OAuthStatePayload{}, err
	}
	return payload, nil
}
