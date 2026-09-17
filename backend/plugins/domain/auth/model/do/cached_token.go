// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package do provides domain data objects for the auth plugin.
package do

// CachedToken represents the minimal cached representation of an access token.
type CachedToken struct {
	ID      uint64 `json:"id"`
	UserID  uint64 `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
}
