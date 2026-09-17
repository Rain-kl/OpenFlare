// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"gorm.io/gorm"
)

var (
	dbMu   sync.RWMutex
	dbSvc  contracts.DBService
	chConn driver.Conn
	chDB   *gorm.DB
)

// SetDBService configures the DBService instance for logstore.
func SetDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

// SetChConnForTest configures ClickHouse native connection for test or runtime.
func SetChConnForTest(conn driver.Conn) {
	dbMu.Lock()
	defer dbMu.Unlock()
	chConn = conn
}

// SetChDBForTest configures ClickHouse GORM DB for test or runtime.
func SetChDBForTest(db *gorm.DB) {
	dbMu.Lock()
	defer dbMu.Unlock()
	chDB = db
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

func getChDB(ctx context.Context) *gorm.DB {
	dbMu.RLock()
	customCh := chDB
	s := dbSvc
	dbMu.RUnlock()
	if customCh != nil {
		return customCh.WithContext(ctx)
	}
	if s != nil {
		if ch := s.Named("clickhouse"); ch != nil {
			return ch.WithContext(ctx)
		}
	}
	return nil
}

func getChConn() driver.Conn {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return chConn
}
