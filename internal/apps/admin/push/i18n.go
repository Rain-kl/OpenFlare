// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/i18n"
)

func localizeEventMetadata(locale string, meta EventMetadata) EventMetadata {
	meta.Name = i18n.Translate(locale, meta.Name)
	meta.Description = i18n.Translate(locale, meta.Description)
	meta.DefaultTemplate.Title = i18n.Translate(locale, meta.DefaultTemplate.Title)
	meta.DefaultTemplate.Content = i18n.Translate(locale, meta.DefaultTemplate.Content)
	return meta
}

func localizePushEvent(locale string, event model.PushEvent) model.PushEvent {
	if meta, found := findBuiltInEvent(event.EventKey); found {
		if event.Name == meta.Name || i18n.Translate("en", event.Name) == meta.Name {
			event.Name = i18n.Translate(locale, meta.Name)
		}
	} else if meta := task.GetTaskMetaByAsynqTask(event.TaskType); meta != nil {
		prefix := "Task completed: "
		english := prefix + meta.Name
		chinese := i18n.Translate("zh-CN", prefix) + i18n.Translate("zh-CN", meta.Name)
		if event.Name == english || event.Name == chinese {
			event.Name = i18n.Translate(locale, prefix) + i18n.Translate(locale, meta.Name)
		}
	}
	// Templates and targets are administrator-owned data, even when they happen
	// to match a built-in message. Never rewrite them during presentation.
	return event
}
