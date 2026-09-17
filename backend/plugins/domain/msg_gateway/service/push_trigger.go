// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	pkgpush "Wavelet/plugins/domain/msg_gateway/push"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// EventTrigger represents the unified event trigger class.
type EventTrigger struct{}

// DefaultTrigger is the singleton instance of EventTrigger.
var DefaultTrigger = &EventTrigger{}

// Trigger receives event metadata and processes the event notification dispatch asynchronously.
func (t *EventTrigger) Trigger(ctx context.Context, meta do.EventMetadata, body map[string]any) {
	asyncCtx := context.WithoutCancel(ctx)
	util.Go(func() {
		if body == nil {
			body = make(map[string]any)
		}
		if _, hasUser := body["user"]; !hasUser || body["user"] == nil {
			body["user"] = GetSystemUser(asyncCtx)
		}

		eventPtr, err := dao.GetActivePushEventByKey(asyncCtx, meta.Key)
		if err != nil {
			if errors.Is(err, consts.ErrRecordNotFound) {
				return
			}
			logger.ErrorF(asyncCtx, "push_event_trigger: failed to get active event %s: %v", meta.Key, err)
			return
		}
		event := *eventPtr
		if len(event.Channels) == 0 {
			return
		}

		flatBody := GetFlatBody(body)
		msg, _ := t.buildMessage(&event, meta, flatBody, body)
		t.enqueuePushTasks(asyncCtx, meta, &event, msg, flatBody)
	})
}

func (t *EventTrigger) buildMessage(event *entity.PushEvent, meta do.EventMetadata, flatBody, body map[string]any) (do.NotificationMessage, string) {
	var msg do.NotificationMessage
	renderedTemplate := ""

	templateSource := event.Template
	if templateSource != "" {
		var err error
		msg, renderedTemplate, err = t.parseCustomTemplate(event, templateSource, flatBody)
		if err != nil {
			msg.Title = event.Name
			msg.Content = renderedTemplate
			msg.Level = consts.DefaultLevelInfo
		}
	} else {
		msg = t.parseDefaultTemplate(meta, flatBody)
	}

	if msg.Ext == nil {
		msg.Ext = make(map[string]any)
	}
	for k, v := range body {
		if k == consts.KeyTitle || k == consts.KeyContent || k == consts.KeyLevel {
			continue
		}
		if _, exists := msg.Ext[k]; !exists {
			msg.Ext[k] = v
		}
	}

	return msg, renderedTemplate
}

func (t *EventTrigger) parseCustomTemplate(event *entity.PushEvent, templateSource string, flatBody map[string]any) (do.NotificationMessage, string, error) {
	var msg do.NotificationMessage
	renderedTemplate := pkgpush.ParseTemplate(templateSource, flatBody)

	var tMap map[string]any
	if err := json.Unmarshal([]byte(renderedTemplate), &tMap); err != nil {
		return msg, renderedTemplate, err
	}

	if title, ok := tMap[consts.KeyTitle].(string); ok && title != "" {
		msg.Title = title
	} else {
		msg.Title = event.Name
	}
	delete(tMap, consts.KeyTitle)

	if content, ok := tMap[consts.KeyContent].(string); ok && content != "" {
		msg.Content = content
	} else {
		msg.Content = renderedTemplate
	}
	delete(tMap, consts.KeyContent)

	if level, ok := tMap[consts.KeyLevel].(string); ok && level != "" {
		msg.Level = level
	} else {
		msg.Level = consts.DefaultLevelInfo
	}
	delete(tMap, consts.KeyLevel)

	msg.Ext = tMap
	return msg, renderedTemplate, nil
}

func (t *EventTrigger) parseDefaultTemplate(meta do.EventMetadata, flatBody map[string]any) do.NotificationMessage {
	var msg do.NotificationMessage
	msg.Title = pkgpush.ParseTemplate(meta.DefaultTemplate.Title, flatBody)
	msg.Content = pkgpush.ParseTemplate(meta.DefaultTemplate.Content, flatBody)
	msg.Level = pkgpush.ParseTemplate(meta.DefaultTemplate.Level, flatBody)

	if meta.DefaultTemplate.Ext != nil {
		msg.Ext = make(map[string]any)
		for k, v := range meta.DefaultTemplate.Ext {
			if strVal, ok := v.(string); ok {
				msg.Ext[k] = pkgpush.ParseTemplate(strVal, flatBody)
			} else {
				msg.Ext[k] = v
			}
		}
	}
	return msg
}

func (t *EventTrigger) enqueuePushTasks(ctx context.Context, meta do.EventMetadata, event *entity.PushEvent, msg do.NotificationMessage, flatBody map[string]any) {
	for _, channelName := range event.Channels {
		customChannel, err := dao.GetActivePushChannelByName(ctx, channelName)
		if err == nil {
			t.enqueueCustomPushChannelTasks(ctx, meta, event, customChannel, msg, flatBody)
			continue
		}
		logger.WarnF(ctx, "push_event_trigger: channel %q not found in DB or disabled: %v", channelName, err)
	}
}

func (t *EventTrigger) enqueueCustomPushChannelTasks(ctx context.Context, meta do.EventMetadata, event *entity.PushEvent, channel *entity.PushChannel, msg do.NotificationMessage, flatBody map[string]any) {
	if len(event.Targets) == 0 {
		t.enqueueSingleCustomPushChannelTask(ctx, meta, channel, "", msg)
		return
	}

	for _, target := range event.Targets {
		resolvedTarget := ResolveTarget(ctx, target, flatBody, channel.Name)
		t.enqueueSingleCustomPushChannelTask(ctx, meta, channel, resolvedTarget, msg)
	}
}

func (t *EventTrigger) enqueueSingleCustomPushChannelTask(ctx context.Context, meta do.EventMetadata, channel *entity.PushChannel, target string, msg do.NotificationMessage) {
	var config pkgpush.Config
	var renderedTemplate string

	switch channel.Type {
	case consts.ChannelLark:
		config = pkgpush.Config{Channel: consts.ChannelLark, URL: channel.URL, Secret: channel.Token}
		renderedTemplate = channel.Other
	case consts.ChannelEmail:
		config = pkgpush.Config{Channel: consts.ChannelEmail, URL: channel.URL, Key: channel.Token, Secret: channel.Other}
	case consts.ChannelTelegram:
		config = pkgpush.Config{Channel: consts.ChannelTelegram, URL: channel.URL, Secret: channel.Token, Key: channel.Other}
	default:
		config = pkgpush.Config{Channel: consts.ChannelCustom, URL: channel.URL}
		customPushReq := do.CustomPushRequest{
			Title:       msg.Title,
			Content:     msg.Content,
			Description: meta.Description,
			To:          target,
		}
		if urlVal, ok := msg.Ext["url"].(string); ok {
			customPushReq.URL = urlVal
		}
		renderedTemplate = RenderCustomPayload(channel.Other, customPushReq)
	}

	payload := do.SendPayload{
		EventKey: meta.Key,
		Config:   config,
		Target:   target,
		Body:     msg,
		Template: renderedTemplate,
	}
	if err := EnqueuePushTask(ctx, payload); err != nil {
		logger.ErrorF(ctx, "push_event_trigger: enqueuePushTask failed for %s channel %s -> %s: %v", channel.Type, channel.Name, target, err)
	}
}

// ResolveTarget parses dynamic placeholders into concrete receiver targets.
func ResolveTarget(ctx context.Context, target string, flatBody map[string]any, channel string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	resolved := ResolveDynamicKeyword(target, flatBody)
	if strings.Contains(resolved, "@") {
		return resolved
	}
	if val, matched := ResolveSystemTarget(ctx, resolved, channel); matched {
		return val
	}

	user, found := ResolveTargetUser(ctx, resolved, channel)
	if !found {
		return resolved
	}
	if channel == consts.ChannelEmail && user.Email != "" {
		return user.Email
	}
	if channel != consts.ChannelEmail && user.Username != "" {
		return user.Username
	}
	return resolved
}

// ResolveDynamicKeyword resolves user.id, username, email keywords.
func ResolveDynamicKeyword(target string, flatBody map[string]any) string {
	switch target {
	case "user.id", "id":
		if val, ok := flatBody["user.id"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["id"]; ok {
			return fmt.Sprintf("%v", val)
		}
	case "user.username", "username":
		if val, ok := flatBody["user.username"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["username"]; ok {
			return fmt.Sprintf("%v", val)
		}
	case "user.email", consts.ChannelEmail:
		if val, ok := flatBody["user.email"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["email"]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return target
}

// ResolveTargetUser resolves user by numeric ID or username string via UserService contract.
func ResolveTargetUser(ctx context.Context, resolved, _ string) (contracts.UserDTO, bool) {
	if id, err := strconv.ParseUint(resolved, 10, 64); err == nil {
		if u, err := FindUserByID(ctx, id); err == nil && u != nil {
			return *u, true
		}
	}
	if u, err := FindUserByUsername(ctx, resolved); err == nil && u != nil {
		return *u, true
	}
	return contracts.UserDTO{}, false
}

// FindUserByID resolves a user by primary key through UserService.
func FindUserByID(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	if userSvc := GetUserService(ctx); userSvc != nil {
		return userSvc.GetUserByID(ctx, id)
	}
	return nil, consts.ErrUserNotFound
}

// FindUserByUsername resolves a user by login name through UserService.
func FindUserByUsername(ctx context.Context, username string) (*contracts.UserDTO, error) {
	if userSvc := GetUserService(ctx); userSvc != nil {
		return userSvc.GetUserByUsername(ctx, username)
	}
	return nil, consts.ErrUserNotFound
}

// GetFirstAdminUser resolves the first administrator through the UserService contract.
func GetFirstAdminUser(ctx context.Context) (*contracts.UserDTO, error) {
	if userSvc := GetUserService(ctx); userSvc != nil {
		return userSvc.GetFirstAdminUser(ctx)
	}
	return nil, consts.ErrNoAdminUser
}

// ResolveSystemTarget maps system receiver aliases to administrator contact info.
func ResolveSystemTarget(ctx context.Context, resolved, channel string) (string, bool) {
	if resolved != "系统" && resolved != "system" && resolved != "0" {
		return "", false
	}
	adminUser, err := GetFirstAdminUser(ctx)
	if err != nil || adminUser == nil {
		return resolved, true
	}
	if channel == consts.ChannelEmail && adminUser.Email != "" {
		return adminUser.Email, true
	}
	if channel != consts.ChannelEmail && adminUser.Username != "" {
		return adminUser.Username, true
	}
	return resolved, true
}

// GetSystemUser gets a system user DTO.
func GetSystemUser(ctx context.Context) *contracts.UserDTO {
	if adminUser, err := GetFirstAdminUser(ctx); err == nil && adminUser != nil {
		return adminUser
	}
	return &contracts.UserDTO{
		Username: "system",
		Nickname: "系统管理员",
	}
}

// LoadUserFromPayload extracts user info from data.
func LoadUserFromPayload(ctx context.Context, data map[string]any) any {
	if u, exists := data["user"]; exists && u != nil {
		return u
	}

	if userID, ok := ExtractUserID(data); ok && userID > 0 {
		if user, err := FindUserByID(ctx, userID); err == nil && user != nil {
			return user
		}
	}

	if username := ExtractUsername(data); username != "" {
		if user, err := FindUserByUsername(ctx, username); err == nil && user != nil {
			return user
		}
	}
	return nil
}

// ExtractUserFromMap extracts user info from payload/detail into body map.
func ExtractUserFromMap(ctx context.Context, data, body map[string]any) {
	if u, exists := body["user"]; exists && u != nil {
		return
	}
	if user := LoadUserFromPayload(ctx, data); user != nil {
		body["user"] = user
	}
}

// ExtractUserID extracts a user ID from map keys.
func ExtractUserID(data map[string]any) (uint64, bool) {
	for _, k := range []string{"user_id", "userId", "uid"} {
		val, ok := data[k]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case float64:
			if v >= 0 {
				return uint64(v), true
			}
		case int:
			if v >= 0 {
				return uint64(v), true
			}
		case int64:
			if v >= 0 {
				return uint64(v), true
			}
		case uint64:
			return v, true
		case string:
			if id, err := strconv.ParseUint(v, 10, 64); err == nil {
				return id, true
			}
		}
	}
	return 0, false
}

// ExtractUsername extracts a username string from map keys.
func ExtractUsername(data map[string]any) string {
	for _, k := range []string{"username", "user_name"} {
		if val, ok := data[k]; ok && val != nil {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
