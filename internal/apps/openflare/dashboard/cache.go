// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"sync"
	"time"
)

const overviewCacheTTL = 30 * time.Second

var overviewCache struct {
	mu        sync.Mutex
	payload   *OverviewPayload
	expiresAt time.Time
}

func getCachedOverview() (*OverviewPayload, bool) {
	overviewCache.mu.Lock()
	defer overviewCache.mu.Unlock()
	if overviewCache.payload == nil || time.Now().After(overviewCache.expiresAt) {
		return nil, false
	}
	return overviewCache.payload, true
}

func setCachedOverview(payload *OverviewPayload) {
	overviewCache.mu.Lock()
	defer overviewCache.mu.Unlock()
	overviewCache.payload = payload
	overviewCache.expiresAt = time.Now().Add(overviewCacheTTL)
}
