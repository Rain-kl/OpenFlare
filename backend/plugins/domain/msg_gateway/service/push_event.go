// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	builtInEventsMu sync.RWMutex
	// BuiltInEvents lists all built-in events defined across the domain.
	BuiltInEvents []do.EventMetadata
)

// RegisterBuiltInEvent registers a built-in event definition.
func RegisterBuiltInEvent(meta do.EventMetadata) {
	builtInEventsMu.Lock()
	defer builtInEventsMu.Unlock()
	for i, e := range BuiltInEvents {
		if e.Key == meta.Key {
			BuiltInEvents[i] = meta
			return
		}
	}
	BuiltInEvents = append(BuiltInEvents, meta)
}

// GetBuiltInEvents returns a copy of registered built-in events.
func GetBuiltInEvents() []do.EventMetadata {
	builtInEventsMu.RLock()
	defer builtInEventsMu.RUnlock()
	out := make([]do.EventMetadata, len(BuiltInEvents))
	copy(out, BuiltInEvents)
	return out
}

// FindBuiltInEvent finds a registered built-in event by key.
func FindBuiltInEvent(key string) (do.EventMetadata, bool) {
	for _, meta := range GetBuiltInEvents() {
		if meta.Key == key {
			return meta, true
		}
	}
	return do.EventMetadata{}, false
}

// PushRegistryAdapter adapts contracts.PushRegistry onto the built-in event store.
type PushRegistryAdapter struct{}

// RegisterBuiltInEvent records a built-in push event definition from cross-plugin contract.
func (PushRegistryAdapter) RegisterBuiltInEvent(meta contracts.PushEventMeta) {
	RegisterBuiltInEvent(eventMetadataFromContract(meta))
}

// SyncEvents persists registered built-in events into the database.
func (PushRegistryAdapter) SyncEvents(ctx context.Context) error {
	return SyncEvents(ctx)
}

func eventMetadataFromContract(meta contracts.PushEventMeta) do.EventMetadata {
	return do.EventMetadata{
		Key:         meta.Key,
		Name:        meta.Name,
		Description: meta.Description,
		DefaultTemplate: do.NotificationMessage{
			Title:   meta.DefaultTemplate.Title,
			Content: meta.DefaultTemplate.Content,
			Level:   meta.DefaultTemplate.Level,
			Ext:     meta.DefaultTemplate.Ext,
		},
	}
}

// SyncBuiltInEvents seeds a database row for every registered built-in event.
func SyncBuiltInEvents(ctx context.Context) error {
	for _, meta := range GetBuiltInEvents() {
		_, err := dao.GetPushEventByKeyRecord(ctx, meta.Key)
		if errors.Is(err, consts.ErrRecordNotFound) {
			var defaultTemplateStr string
			if defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate); err == nil {
				defaultTemplateStr = string(defaultTemplateBytes)
			}
			event := entity.PushEvent{
				EventKey: meta.Key,
				Name:     meta.Name,
				Channels: []string{},
				Targets:  []string{},
				Template: defaultTemplateStr,
				Enabled:  false,
			}
			if err := dao.CreatePushEventRecord(ctx, &event); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// SyncEvents automatically registers/updates built-in events in the database.
func SyncEvents(ctx context.Context) error {
	return SyncBuiltInEvents(ctx)
}

// ListPushEvents lists all configured push events.
func ListPushEvents(ctx context.Context) ([]entity.PushEvent, error) {
	return dao.ListPushEventsRecord(ctx)
}

// CreatePushEvent stores a push event configuration for a built-in event or task type.
func CreatePushEvent(ctx context.Context, req do.CreatePushEventRequest) (entity.PushEvent, error) {
	eventKey, eventName, defaultTemplateBytes, err := GetEventInfo(ctx, req)
	if err != nil {
		return entity.PushEvent{}, err
	}

	count, err := dao.CountPushEventsByKeyRecord(ctx, eventKey)
	if err != nil {
		return entity.PushEvent{}, err
	}
	if count > 0 {
		return entity.PushEvent{}, errors.New(consts.ErrEventAlreadyConfigured)
	}

	templateStr := strings.TrimSpace(req.Template)
	if templateStr == "" {
		templateStr = string(defaultTemplateBytes)
	} else {
		var tempMap map[string]any
		if err := json.Unmarshal([]byte(templateStr), &tempMap); err != nil {
			return entity.PushEvent{}, errors.New(consts.ErrTemplateInvalidJSON)
		}
	}

	channels := req.Channels
	if channels == nil {
		channels = []string{}
	}
	targets := req.Targets
	if targets == nil {
		targets = []string{}
	}

	event := entity.PushEvent{
		EventKey: eventKey,
		Name:     eventName,
		TaskType: req.TaskType,
		Channels: channels,
		Targets:  targets,
		Template: templateStr,
		Enabled:  req.Enabled,
	}
	if err := event.Validate(); err != nil {
		return entity.PushEvent{}, err
	}
	if err := dao.CreatePushEventRecord(ctx, &event); err != nil {
		return entity.PushEvent{}, err
	}
	return event, nil
}

// DeletePushEvent deletes a push event configuration by id.
func DeletePushEvent(ctx context.Context, id uint64) error {
	event, err := dao.GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return err
	}
	return dao.DeletePushEventRecord(ctx, &event)
}

// UpdatePushEvent replaces mutable push event fields.
func UpdatePushEvent(ctx context.Context, id uint64, req do.UpdatePushEventRequest) error {
	event, err := dao.GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return err
	}

	event.Channels = req.Channels
	event.Targets = req.Targets
	event.Template = req.Template
	event.Enabled = req.Enabled
	if err := event.Validate(); err != nil {
		return err
	}
	return dao.SavePushEventRecord(ctx, &event)
}

// TogglePushEvent flips the enabled flag of a push event.
func TogglePushEvent(ctx context.Context, id uint64) (bool, error) {
	event, err := dao.GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return false, err
	}

	enabled := !event.Enabled
	if enabled && len(event.Channels) == 0 {
		return false, errors.New(consts.ErrEnableWithoutChannels)
	}
	if err := dao.UpdatePushEventEnabledRecord(ctx, &event, enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

// ListActivePushEventsByTaskType returns enabled push events for a given task type.
func ListActivePushEventsByTaskType(ctx context.Context, taskType string) ([]entity.PushEvent, error) {
	return dao.ListActivePushEventsByTaskTypeRecord(ctx, taskType)
}

// GetEventInfo derives the event key, display name and default template for a
// task-completion based event or a registered built-in event key.
func GetEventInfo(ctx context.Context, req do.CreatePushEventRequest) (string, string, []byte, error) {
	if req.TaskType != "" {
		taskName := req.TaskType
		if taskSvc := GetTaskService(ctx); taskSvc != nil {
			if meta, ok := taskSvc.GetTaskMeta(req.TaskType); ok {
				taskName = meta.DisplayName
			}
		}
		eventKey := "task_completed:" + req.TaskType
		eventName := "任务完成: " + taskName
		defaultTemplate := do.NotificationMessage{
			Title:   "任务完成: " + taskName,
			Content: "异步任务 {{task_name}} (ID: {{task_id}}) 已完成。状态: {{task_status}}，耗时: {{task_duration}} ms。",
			Level:   consts.DefaultLevelInfo,
		}
		defaultTemplateBytes, err := json.Marshal(defaultTemplate)
		if err != nil {
			return "", "", nil, err
		}
		return eventKey, eventName, defaultTemplateBytes, nil
	}

	if req.EventKey == "" {
		return "", "", nil, errors.New(consts.ErrEventKeyOrTaskType)
	}

	meta, found := FindBuiltInEvent(req.EventKey)
	if !found {
		return "", "", nil, errors.New(consts.ErrUnsupportedEventKey)
	}

	defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate)
	if err != nil {
		return "", "", nil, err
	}
	return req.EventKey, meta.Name, defaultTemplateBytes, nil
}

// AdminLogin is the metadata definition for the admin login event.
var AdminLogin = do.EventMetadata{
	Key:  "admin_login",
	Name: "管理员登录",
	DefaultTemplate: do.NotificationMessage{
		Title:   "管理员登录提醒",
		Content: "管理员 {{user.username}} 于 {{time}} 从 IP {{ip}} 登录系统。",
		Level:   consts.DefaultLevelInfo,
	},
	Description: "当管理员成功登录系统时触发此通知",
}

// HandleAdminLoggedIn 处理管理员登录事件并触发通知
func HandleAdminLoggedIn(ctx context.Context, event contracts.AdminLoggedIn) {
	if event.User == nil {
		return
	}

	body := map[string]any{
		"user": event.User,
		"ip":   event.IP,
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	DefaultTrigger.Trigger(ctx, AdminLogin, body)
}

// HandleTaskCompleted handles task completion notifications.
func HandleTaskCompleted(ctx context.Context, e contracts.TaskCompletedEvent) {
	events, err := ListActivePushEventsByTaskType(ctx, e.TaskType)
	if err != nil {
		logger.ErrorF(ctx, "push_task_completed_listener: failed to query push events for task type %s: %v", e.TaskType, err)
		return
	}
	if len(events) == 0 {
		return
	}

	body := map[string]any{
		"task_id":       e.TaskID,
		"task_name":     e.TaskName,
		"task_type":     e.TaskType,
		"task_status":   e.Status,
		"task_duration": e.Duration,
		"time":          time.Now().Format("2006-01-02 15:04:05"),
		"task_error":    e.ErrorMsg,
		"task_result":   e.ResultMsg,
	}

	var payloadMap map[string]any
	if e.Payload != "" {
		if err := json.Unmarshal([]byte(e.Payload), &payloadMap); err == nil {
			body["payload"] = payloadMap
			ExtractUserFromMap(ctx, payloadMap, body)
		}
	}
	if e.Detail != "" {
		var detailMap map[string]any
		if err := json.Unmarshal([]byte(e.Detail), &detailMap); err == nil {
			body["detail"] = detailMap
			ExtractUserFromMap(ctx, detailMap, body)
		}
	}

	for _, event := range events {
		meta := do.EventMetadata{
			Key:         event.EventKey,
			Name:        event.Name,
			Description: "异步任务执行完毕触发的自动通知",
		}
		DefaultTrigger.Trigger(ctx, meta, body)
	}
}

// RegisterCustomEvents registers default domain push notification events.
func RegisterCustomEvents() {
	RegisterBuiltInEvent(AdminLogin)
}
