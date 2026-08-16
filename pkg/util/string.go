// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import "strings"

// TrimStringFields trims leading and trailing spaces from all provided string pointers.
func TrimStringFields(fields ...*string) {
	for _, f := range fields {
		if f != nil {
			*f = strings.TrimSpace(*f)
		}
	}
}
