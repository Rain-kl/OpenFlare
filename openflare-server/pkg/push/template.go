// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ParseTemplate parses template strings by replacing {{placeholder}} structures with values from body.
// It is a single-pass parser designed for high performance and low allocations.
func ParseTemplate(template string, body map[string]any) string {
	var buf strings.Builder
	buf.Grow(len(template))

	i := 0
	for {
		pos := strings.Index(template[i:], "{{")
		if pos == -1 {
			buf.WriteString(template[i:])
			break
		}
		// Write prefix
		buf.WriteString(template[i : i+pos])
		i += pos + 2 // skip "{{"

		endPos := strings.Index(template[i:], "}}")
		if endPos == -1 {
			// Unbalanced "{{"
			buf.WriteString("{{")
			buf.WriteString(template[i:])
			break
		}
		key := template[i : i+endPos]
		if val, ok := body[key]; ok {
			buf.WriteString(formatValue(val))
		} else {
			// Keep the placeholder if key not found
			buf.WriteString("{{")
			buf.WriteString(key)
			buf.WriteString("}}")
		}
		i += endPos + 2 // skip "}}"
	}
	return buf.String()
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return strconv.Itoa(val)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		// If it's a map, slice, or struct, marshal it to JSON.
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}
