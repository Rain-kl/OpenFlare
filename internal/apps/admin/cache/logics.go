// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/infra/diskcache"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func applyCacheConfig(ctx context.Context, req updateCacheConfigRequest) error {
	updates := []struct {
		key   string
		value string
	}{
		{model.ConfigKeyDiskCacheMaxSizeMB, strconv.FormatInt(req.MaxSizeMB, 10)},
		{model.ConfigKeyDiskCacheTTLMinutes, strconv.FormatInt(req.TTLMinutes, 10)},
		{model.ConfigKeyDiskCacheLRUEnabled, strconv.FormatBool(req.LRUEnabled)},
	}
	for _, u := range updates {
		if err := repository.SaveOrUpdateSystemConfig(ctx, u.key, u.value); err != nil {
			return err
		}
	}
	diskcache.GetGlobalCache().ReloadConfig(ctx)
	return nil
}
