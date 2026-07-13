// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package batchwriter

import "errors"

var errNilFlushFunc = errors.New("batchwriter: flush func is required")
