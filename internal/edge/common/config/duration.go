// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config provides shared configuration types for edge applications.
package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MillisecondDuration is a time.Duration that marshals to/from JSON as an integer number of milliseconds
// or as a Go duration string (e.g. "1s", "500ms").
type MillisecondDuration time.Duration

// Duration returns the underlying time.Duration value.
func (d MillisecondDuration) Duration() time.Duration {
	return time.Duration(d)
}

func (d MillisecondDuration) String() string {
	return time.Duration(d).String()
}

// UnmarshalJSON decodes either a numeric millisecond value or a quoted Go duration string.
func (d *MillisecondDuration) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*d = 0
		return nil
	}
	if strings.HasPrefix(raw, "\"") {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			*d = 0
			return nil
		}
		parsed, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", text, err)
		}
		*d = MillisecondDuration(parsed)
		return nil
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid duration milliseconds %q: %w", raw, err)
	}
	*d = MillisecondDuration(time.Duration(ms) * time.Millisecond)
	return nil
}

// MarshalJSON encodes the duration as an integer number of milliseconds.
func (d MillisecondDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).Milliseconds())
}
