// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"sync"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/model"
)

const (
	fallbackSystemUserID uint64 = 999
	configTypeSystem            = "system"
)

var (
	taskMu  sync.RWMutex
	taskSvc contracts.TaskService
)

// SetTaskService injects the platform TaskService.
func SetTaskService(s contracts.TaskService) {
	taskMu.Lock()
	defer taskMu.Unlock()
	taskSvc = s
}

func currentTaskService() contracts.TaskService {
	taskMu.RLock()
	defer taskMu.RUnlock()
	return taskSvc
}

// GetActiveAuthSources lists enabled Wavelet auth sources via AuthService.
func GetActiveAuthSources(ctx context.Context) ([]model.AuthSource, error) {
	svc := currentAuthService()
	if svc == nil {
		return nil, errors.New("auth service not initialized")
	}
	views, err := svc.ListAuthSources(ctx)
	if err != nil {
		return nil, err
	}
	sources := make([]model.AuthSource, 0, len(views))
	for _, view := range views {
		if !view.IsActive {
			continue
		}
		sources = append(sources, model.AuthSource{
			ID:          view.ID,
			Name:        view.Name,
			Type:        view.Type,
			DisplayName: view.DisplayName,
			IconURL:     view.IconURL,
			IsActive:    true,
		})
	}
	return sources, nil
}

// GetTaskExecutionByTaskID loads a task execution by public task ID.
func GetTaskExecutionByTaskID(ctx context.Context, taskID string) (*contracts.TaskExecutionDTO, error) {
	svc := currentTaskService()
	if svc == nil {
		return nil, errors.New("task service not initialized")
	}
	return svc.GetExecutionByTaskID(ctx, taskID)
}

// GetSystemUser loads the built-in system user via UserService, or a synthetic fallback.
func GetSystemUser(ctx context.Context) model.User {
	if svc := currentUserService(); svc != nil {
		if user, err := svc.GetUserByUsername(ctx, configTypeSystem); err == nil && user != nil {
			return model.User{
				ID:       user.ID,
				Username: user.Username,
				Nickname: user.Nickname,
				IsActive: user.IsActive,
			}
		}
	}
	return model.User{
		ID:       fallbackSystemUserID,
		Username: configTypeSystem,
		Nickname: "系统",
	}
}
