// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package testhelper

// RegisterCleanup registers an extra cleanup hook invoked by SetupTestEnvironment.
func RegisterCleanup(fn func()) {
	extraCleanups = append(extraCleanups, fn)
}

var extraCleanups []func()

func runExtraCleanups() {
	for _, fn := range extraCleanups {
		fn()
	}
}
