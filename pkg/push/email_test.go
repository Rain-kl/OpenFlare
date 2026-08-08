// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import "testing"

func TestSanitizeEmailHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "System Notification", "System Notification"},
		{"crlf stripped", "alert\r\nBcc: attacker@example.com", "alertBcc: attacker@example.com"},
		{"cr stripped", "a\rb", "ab"},
		{"lf stripped", "a\nb", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeEmailHeader(tt.input); got != tt.want {
				t.Errorf("sanitizeEmailHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
