// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"

	"gorm.io/gorm"
)

var (
	dbMu  sync.RWMutex
	dbSvc contracts.DBService
)

func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
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
