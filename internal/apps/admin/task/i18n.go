// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"slices"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/i18n"
)

// Localize copies, never the shared registry or persistent task records.
func localizeTaskMeta(locale string, meta task.TaskMeta) task.TaskMeta {
	meta.Name = i18n.Translate(locale, meta.Name)
	meta.Description = i18n.Translate(locale, meta.Description)
	meta.Params = slices.Clone(meta.Params)
	for index := range meta.Params {
		param := &meta.Params[index]
		param.Label = i18n.Translate(locale, param.Label)
		param.Placeholder = i18n.Translate(locale, param.Placeholder)
		param.Description = i18n.Translate(locale, param.Description)
	}
	return meta
}

func localizeTaskExecution(locale string, execution model.TaskExecution) model.TaskExecution {
	execution.TaskName = i18n.Translate(locale, execution.TaskName)
	execution.Log = i18n.TranslateLog(locale, execution.Log)
	execution.ErrorMessage = i18n.Translate(locale, execution.ErrorMessage)
	execution.Result = i18n.Translate(locale, execution.Result)
	return execution
}

func localizeTaskExecutions(locale string, executions []model.TaskExecution) []model.TaskExecution {
	localized := slices.Clone(executions)
	for index := range localized {
		localized[index] = localizeTaskExecution(locale, localized[index])
	}
	return localized
}

func localizeScheduleName(locale, name string, meta *task.TaskMeta) string {
	// Only the built-in default name is system text. Custom schedule names are data.
	if meta != nil && meta.Type == "system_cleanup" && i18n.Translate("en", name) == "Scheduled system cleanup" {
		return i18n.Translate(locale, "Scheduled system cleanup")
	}
	if meta != nil && i18n.Translate("en", name) == meta.Name {
		return i18n.Translate(locale, meta.Name)
	}
	return name
}
