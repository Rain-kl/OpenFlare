// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao_test

import (
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBotDAO_ChannelAndBinding(t *testing.T) {
	_ = idgen.Init(1)
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.AutoMigrate(&entity.MessageChannel{}, &entity.MessageBinding{}, &entity.MessagePairingCode{}))

	dao.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	ctx := context.Background()

	ch := entity.MessageChannel{
		Name:        "tg_bot",
		Type:        "telegram",
		OwnerScope:  "system",
		Credentials: "encrypted_token",
		Enabled:     true,
	}
	require.NoError(t, dao.CreateMessageChannel(ctx, &ch))
	assert.NotZero(t, ch.ID)

	code, err := dao.UpsertPairingCode(ctx, ch.ID, "tg_user_1", "ABCD1234", time.Now().Add(10*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "ABCD1234", code.Code)

	// Reusing pairing code
	code2, err := dao.UpsertPairingCode(ctx, ch.ID, "tg_user_1", "XYZ9999", time.Now().Add(10*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "ABCD1234", code2.Code)

	binding := entity.MessageBinding{
		UserID:         42,
		ChannelID:      ch.ID,
		PlatformUserID: "tg_user_1",
	}
	require.NoError(t, dao.CreateMessageBinding(ctx, &binding))
	assert.NotZero(t, binding.ID)

	bindings, err := dao.ListBindingsByUser(ctx, 42)
	require.NoError(t, err)
	assert.Len(t, bindings, 1)

	require.NoError(t, dao.DeleteMessageChannel(ctx, ch.ID))
	_, err = dao.GetMessageChannel(ctx, ch.ID)
	assert.Error(t, err)
}
