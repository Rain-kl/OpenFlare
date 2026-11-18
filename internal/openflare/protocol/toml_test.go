// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

import "testing"

func TestTOMLQuote(t *testing.T) {
	cases := map[string]string{
		`plain`:         `"plain"`,
		`a"b`:           `"a\"b"`,
		`a\b`:           `"a\\b"`,
		"injection\"\n": `"injection\"\n"`,
		"":              `""`,
		"a\tb":          `"a\tb"`,
	}
	for in, want := range cases {
		if got := TOMLQuote(in); got != want {
			t.Errorf("TOMLQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
