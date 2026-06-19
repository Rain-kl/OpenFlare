// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/repository"
)

func saveOrUpdateConfig(ctx context.Context, key, value string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, key, value)
}
