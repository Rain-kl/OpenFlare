// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"Wavelet/plugins/domain/msg_gateway/service"
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type dispatchTestDB struct{ db *gorm.DB }

func (m *dispatchTestDB) GORM() *gorm.DB                  { return m.db }
func (m *dispatchTestDB) DB(ctx context.Context) *gorm.DB { return m.db.WithContext(ctx) }
func (m *dispatchTestDB) Named(_ string) *gorm.DB         { return m.db }

func TestBotDispatchValidatePayload(t *testing.T) {
	h := &service.BotDispatchHandler{}
	_, err := h.ValidatePayload([]byte(`{}`))
	require.Error(t, err)
	_, err = h.ValidatePayload([]byte(`{"text":"hello"}`))
	require.NoError(t, err)
}

func TestBotDispatchNoChannels(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dispatch.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&entity.MessageChannel{}, &entity.MessageBinding{}))
	dao.SetDBServiceForTest(&dispatchTestDB{db: testDB})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	h := &service.BotDispatchHandler{}
	res, err := h.Execute(context.Background(), []byte(`{"text":"hello"}`))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.Message, "成功 0")
}
