//go:build !unix

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"context"
	"errors"
)

func inspectOpenrestyProcesses(_ context.Context, _, _ string) ([]runtimeProcess, error) {
	return nil, errors.New("safe openresty process recovery is unsupported on this platform; refusing an unverified start")
}

func signalOpenrestyProcess(_ context.Context, _, _ string, _ runtimeProcess, _ bool) error {
	return errors.New("safe openresty process signalling is unsupported on this platform")
}
