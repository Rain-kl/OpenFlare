// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import "github.com/Rain-kl/Wavelet/internal/testhelper"

func init() {
	testhelper.RegisterCleanup(ResetRuntimeSettingsForTest)
}
