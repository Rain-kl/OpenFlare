// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestLocalizeEventMetadataPreservesTemplatesAndRegistry(t *testing.T) {
	source := EventMetadata{Name: "Administrator login", DefaultTemplate: NotificationMessage{Title: "Administrator login notification"}}
	localized := localizeEventMetadata("zh-CN", source)
	assert.Equal(t, "管理员登录", localized.Name)
	assert.Equal(t, "管理员登录提醒", localized.DefaultTemplate.Title)
	assert.Equal(t, "Administrator login", source.Name)
	assert.Equal(t, "Administrator login notification", source.DefaultTemplate.Title)

	event := model.PushEvent{Name: "自定义事件", Template: `{"title":"管理员登录提醒"}`, Targets: []string{"系统"}}
	assert.Equal(t, event, localizePushEvent("en", event))
}
