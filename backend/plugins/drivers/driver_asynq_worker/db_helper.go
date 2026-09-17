// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	dbMu      sync.RWMutex
	dbSvc     contracts.DBService
	redisMu   sync.RWMutex
	rdbClient redis.UniversalClient
)

func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

// SetRedisClient sets the redis client used for task logs.
func SetRedisClient(c redis.UniversalClient) {
	redisMu.Lock()
	defer redisMu.Unlock()
	rdbClient = c
}

func getDB(ctx context.Context) *gorm.DB {
	if s, err := core.InjectFrom[contracts.DBService](ctx); err == nil && s != nil {
		return s.DB(ctx)
	}
	dbMu.RLock()
	s := dbSvc
	dbMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}
	return nil
}

func getRedisClient() redis.UniversalClient {
	redisMu.RLock()
	defer redisMu.RUnlock()
	return rdbClient
}
