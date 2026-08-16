// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"strconv"
)

// Interface2String converts a string, int, or float64 value to its string representation.
func Interface2String(inter any) string {
	switch v := inter.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return fmt.Sprintf("%f", v)
	}
	return "Not Implemented"
}
