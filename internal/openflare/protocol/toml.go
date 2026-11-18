// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

import "strings"

// TOMLQuote renders s as a quoted TOML basic string, escaping characters that
// would otherwise break the document or allow key injection (quotes,
// backslashes, control/newline characters). Use it for every interpolated
// value written into frps/frpc TOML configs.
func TOMLQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
