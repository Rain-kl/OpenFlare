// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/task"
	pkgpush "github.com/Rain-kl/Wavelet/pkg/push"
	"gorm.io/gorm"
)

type smtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

func loadSMTPConfig(ctx context.Context) smtpConfig {
	host, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPHost)
	port, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPort)
	user, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPUsername)
	pass, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPassword)
	return smtpConfig{
		Host:     host.Value,
		Port:     port.Value,
		Username: user.Value,
		Password: pass.Value,
	}
}

func syncBuiltInEvents(ctx context.Context) error {
	for _, meta := range BuiltInEvents {
		_, err := repository.GetPushEventByKey(ctx, meta.Key)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var defaultTemplateStr string
			if defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate); err == nil {
				defaultTemplateStr = string(defaultTemplateBytes)
			}
			event := model.PushEvent{
				EventKey: meta.Key,
				Name:     meta.Name,
				Channels: []string{},
				Targets:  []string{},
				Template: defaultTemplateStr,
				Enabled:  false,
			}
			if err := repository.CreatePushEvent(ctx, &event); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

func listPushEvents(ctx context.Context) ([]model.PushEvent, error) {
	return repository.ListPushEvents(ctx)
}

func createPushEvent(ctx context.Context, req CreateEventRequest) (model.PushEvent, error) {
	eventKey, eventName, defaultTemplateBytes, err := getEventInfo(req)
	if err != nil {
		return model.PushEvent{}, err
	}

	count, err := repository.CountPushEventsByKey(ctx, eventKey)
	if err != nil {
		return model.PushEvent{}, err
	}
	if count > 0 {
		return model.PushEvent{}, errors.New("this notification event is already configured")
	}

	templateStr := strings.TrimSpace(req.Template)
	if templateStr == "" {
		templateStr = string(defaultTemplateBytes)
	} else {
		var tempMap map[string]any
		if err := json.Unmarshal([]byte(templateStr), &tempMap); err != nil {
			return model.PushEvent{}, errors.New("custom template is not a valid JSON format")
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

	event := model.PushEvent{
		EventKey: eventKey,
		Name:     eventName,
		TaskType: req.TaskType,
		Channels: channels,
		Targets:  targets,
		Template: templateStr,
		Enabled:  req.Enabled,
	}
	if err := event.Validate(); err != nil {
		return model.PushEvent{}, err
	}
	if err := repository.CreatePushEvent(ctx, &event); err != nil {
		return model.PushEvent{}, err
	}
	return event, nil
}

func deletePushEvent(ctx context.Context, id uint64) error {
	event, err := repository.GetPushEventByID(ctx, id)
	if err != nil {
		return err
	}
	return repository.DeletePushEvent(ctx, &event)
}

func updatePushEvent(ctx context.Context, id uint64, req UpdateEventRequest) error {
	event, err := repository.GetPushEventByID(ctx, id)
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
	return repository.SavePushEvent(ctx, &event)
}

func togglePushEvent(ctx context.Context, id uint64) (bool, error) {
	event, err := repository.GetPushEventByID(ctx, id)
	if err != nil {
		return false, err
	}

	enabled := !event.Enabled
	if enabled && len(event.Channels) == 0 {
		return false, errors.New("cannot enable event without any push channels configured")
	}
	if err := repository.UpdatePushEventEnabled(ctx, &event, enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func listPushHistories(ctx context.Context, filter repository.PushHistoryListFilter) (int64, []model.PushHistory, error) {
	return repository.ListPushHistories(ctx, filter)
}

func applySMTPFallbackToPushConfig(ctx context.Context, cfg *pkgpush.Config) {
	if cfg.Channel != channelEmail || (cfg.URL != "" && cfg.Key != "") {
		return
	}
	smtp := loadSMTPConfig(ctx)
	if smtp.Host == "" || smtp.Username == "" {
		return
	}
	port := smtp.Port
	if port == "" {
		port = "587"
	}
	cfg.URL = smtp.Host + ":" + port
	cfg.Key = smtp.Username
	cfg.Secret = smtp.Password
}

func listPushChannels(ctx context.Context) ([]model.PushChannel, error) {
	return repository.ListPushChannels(ctx)
}

func createPushChannel(ctx context.Context, req CreateChannelRequest) (model.PushChannel, error) {
	count, err := repository.CountPushChannelsByName(ctx, req.Name)
	if err != nil {
		return model.PushChannel{}, err
	}
	if count > 0 {
		return model.PushChannel{}, errors.New("channel name already exists")
	}

	channel := model.PushChannel{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Token:       req.Token,
		URL:         req.URL,
		Other:       req.Other,
		Enabled:     req.Enabled,
	}
	if err := channel.Validate(); err != nil {
		return model.PushChannel{}, err
	}
	if err := repository.CreatePushChannel(ctx, &channel); err != nil {
		return model.PushChannel{}, err
	}
	return channel, nil
}

func updatePushChannel(ctx context.Context, id uint64, req UpdateChannelRequest) (model.PushChannel, error) {
	channel, err := repository.GetPushChannelByID(ctx, id)
	if err != nil {
		return model.PushChannel{}, err
	}

	channel.Description = req.Description
	channel.Type = req.Type
	channel.Token = req.Token
	channel.URL = req.URL
	channel.Other = req.Other
	channel.Enabled = req.Enabled
	if err := channel.Validate(); err != nil {
		return model.PushChannel{}, err
	}
	if err := repository.SavePushChannel(ctx, &channel); err != nil {
		return model.PushChannel{}, err
	}
	return channel, nil
}

func deletePushChannel(ctx context.Context, id uint64) error {
	channel, err := repository.GetPushChannelByID(ctx, id)
	if err != nil {
		return err
	}
	return repository.DeletePushChannel(ctx, &channel)
}

func loadChannelForTest(ctx context.Context, req TestChannelRequest) (string, string, string, string, error) {
	if req.Name != "" {
		channel, err := repository.GetPushChannelByName(ctx, req.Name)
		if err != nil {
			return "", "", "", "", errors.New("channel not found")
		}
		return channel.URL, channel.Token, channel.Other, channel.Type, nil
	}
	return req.URL, req.Token, req.Other, req.Type, nil
}

func listActivePushEventsByTaskType(ctx context.Context, taskType string) ([]model.PushEvent, error) {
	return repository.ListActivePushEventsByTaskType(ctx, taskType)
}

func loadUserFromPayload(ctx context.Context, data map[string]any) any {
	if u, exists := data["user"]; exists && u != nil {
		return u
	}

	if userID, ok := extractUserID(data); ok && userID > 0 {
		if user, err := repository.GetUserByID(ctx, userID); err == nil {
			return &user
		}
	}

	if username := extractUsername(data); username != "" {
		if user, err := repository.GetUserByUsername(ctx, username); err == nil {
			return &user
		}
	}
	return nil
}

func recordPushHistory(ctx context.Context, req SendPayload, status, errMsg string) error {
	title := req.Body.Title
	content := req.Body.Content
	level := req.Body.Level
	if title == "" {
		title = "系统通知"
	}
	if level == "" {
		level = defaultLevelInfo
	}

	target := req.Target
	if target == "" {
		if req.Config.URL != "" {
			target = req.Config.URL
			const maxTargetLen = 50
			const truncatedLen = 47
			if len(target) > maxTargetLen {
				target = target[:truncatedLen] + "..."
			}
		} else {
			target = "default"
		}
	}

	history := model.PushHistory{
		EventKey: req.EventKey,
		Channel:  req.Config.Channel,
		Target:   target,
		Title:    title,
		Content:  content,
		Level:    level,
		Status:   status,
		ErrorMsg: errMsg,
	}
	return repository.CreatePushHistory(ctx, &history)
}

func resolveTargetUser(ctx context.Context, resolved string, _ string) (model.User, bool) {
	found := false
	var user model.User

	if id, err := strconv.ParseUint(resolved, 10, 64); err == nil {
		if u, err := repository.GetUserByID(ctx, id); err == nil {
			user = u
			found = true
		}
	}
	if !found {
		if u, err := repository.GetUserByUsername(ctx, resolved); err == nil {
			user = u
			found = true
		}
	}
	return user, found
}

func resolveSystemTarget(ctx context.Context, resolved string, channel string) (string, bool) {
	if resolved != "系统" && resolved != "system" && resolved != "0" {
		return "", false
	}
	adminUser, err := repository.GetFirstAdminUser(ctx)
	if err != nil {
		return resolved, true
	}
	if channel == channelEmail && adminUser.Email != "" {
		return adminUser.Email, true
	}
	if channel != channelEmail && adminUser.Username != "" {
		return adminUser.Username, true
	}
	return resolved, true
}

func resolveSMTPConfig(ctx context.Context, url, token, other string) (string, string, string) {
	if url != "" && token != "" {
		return url, token, other
	}
	smtp := loadSMTPConfig(ctx)
	if smtp.Host == "" || smtp.Username == "" {
		return url, token, other
	}
	port := smtp.Port
	if port == "" {
		port = "587"
	}
	if url == "" {
		url = smtp.Host + ":" + port
	}
	if token == "" {
		token = smtp.Username
	}
	if other == "" {
		other = smtp.Password
	}
	return url, token, other
}

func getSystemUser(ctx context.Context) *model.User {
	user := repository.GetSystemUser(ctx)
	return &user
}

func getEventInfo(req CreateEventRequest) (string, string, []byte, error) {
	if req.TaskType != "" {
		meta := task.GetTaskMetaByAsynqTask(req.TaskType)
		if meta == nil {
			return "", "", nil, errors.New("unsupported task type")
		}
		eventKey := "task_completed:" + req.TaskType
		eventName := "任务完成: " + meta.Name
		defaultTemplate := NotificationMessage{
			Title:   "任务完成: " + meta.Name,
			Content: "异步任务 {{task_name}} (ID: {{task_id}}) 已完成。状态: {{task_status}}，耗时: {{task_duration}} ms。",
			Level:   defaultLevelInfo,
		}
		defaultTemplateBytes, err := json.Marshal(defaultTemplate)
		if err != nil {
			return "", "", nil, err
		}
		return eventKey, eventName, defaultTemplateBytes, nil
	}

	if req.EventKey == "" {
		return "", "", nil, errors.New("either event_key or task_type must be provided")
	}

	meta, found := findBuiltInEvent(req.EventKey)
	if !found {
		return "", "", nil, errors.New("unsupported built-in event key")
	}

	defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate)
	if err != nil {
		return "", "", nil, err
	}
	return req.EventKey, meta.Name, defaultTemplateBytes, nil
}
