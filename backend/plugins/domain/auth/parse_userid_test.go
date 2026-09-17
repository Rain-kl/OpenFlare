// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestParseUserIDSnowflakeStringPreservesValue(t *testing.T) {
	id := uint64(99835970421002240)
	got := ParseUserID(strconv.FormatUint(id, 10))
	if got != id {
		t.Errorf("ParseUserID(%q) = %d, want %d", strconv.FormatUint(id, 10), got, id)
	}
}

func TestParseUserIDJSONNumberAboveMaxSafeInteger(t *testing.T) {
	// 2^53+1 cannot be represented as a distinct IEEE-754 float64.
	id := uint64(9007199254740993)
	got := ParseUserID(float64(id))
	if got == id {
		t.Fatalf("ParseUserID(float64(%d)) = %d, want a rounded value", id, got)
	}
}

func TestBasicUserInfoJSONEncodesIDAsString(t *testing.T) {
	info := BasicUserInfo{ID: 99835970421002240, Username: "plain_user"}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal(BasicUserInfo) error = %v", err)
	}
	var probe struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("json.Unmarshal probe error = %v", err)
	}
	if len(probe.ID) == 0 || probe.ID[0] != '"' {
		t.Errorf("BasicUserInfo id JSON = %s, want a JSON string", probe.ID)
	}
}
