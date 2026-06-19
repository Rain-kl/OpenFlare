// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package push defines push notification HTTP routes, background tasks, and events.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgpush "github.com/Rain-kl/Wavelet/pkg/push"
	"gorm.io/gorm"
)

// NotificationMessage represents the structured notification message payload.
type NotificationMessage struct {
	Title   string         `json:"title"`
	Content string         `json:"content"`
	Level   string         `json:"level"`
	Ext     map[string]any `json:"ext,omitempty"`
}

// Flatten converts the structured NotificationMessage back to a flat map (original json structure).
func (m NotificationMessage) Flatten() map[string]any {
	res := map[string]any{
		keyTitle:   m.Title,
		keyContent: m.Content,
		keyLevel:   m.Level,
	}
	for k, v := range m.Ext {
		res[k] = v
	}
	return res
}

// EventMetadata represents the metadata of a push notification event.
type EventMetadata struct {
	Key             string              `json:"key"`
	Name            string              `json:"name"`
	DefaultTemplate NotificationMessage `json:"default_template"`
	Description     string              `json:"description"`
}

// SendPayload 异步投递推送载荷 (供 task/Worker 使用)
type SendPayload struct {
	EventKey string              `json:"event_key"`
	Config   pkgpush.Config      `json:"config"`
	Target   string              `json:"target"`
	Body     NotificationMessage `json:"body"`
	Template string              `json:"template"`
}

// BuiltInEvents lists all built-in events defined in custom_events.
var BuiltInEvents []EventMetadata

// RegisterBuiltInEvent registers a built-in event definition.
func RegisterBuiltInEvent(meta EventMetadata) {
	BuiltInEvents = append(BuiltInEvents, meta)
}

// EventTrigger represents the unified event trigger class.
type EventTrigger struct{}

// DefaultTrigger is the singleton instance of EventTrigger.
var DefaultTrigger = &EventTrigger{}

// Trigger receives event metadata and processes the event notification dispatch asynchronously.
//
//nolint:contextcheck
func (t *EventTrigger) Trigger(ctx context.Context, meta EventMetadata, body map[string]any) {
	asyncCtx := context.WithoutCancel(ctx)
	go func() {
		if body == nil {
			body = make(map[string]any)
		}
		if _, hasUser := body["user"]; !hasUser || body["user"] == nil {
			body["user"] = getSystemUser(asyncCtx)
		}

		eventPtr, err := repository.GetActivePushEventByKey(asyncCtx, meta.Key)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}
			logger.ErrorF(asyncCtx, "push_event_trigger: failed to get active event %s: %v", meta.Key, err)
			return
		}
		event := *eventPtr
		if len(event.Channels) == 0 {
			return
		}

		flatBody := getFlatBody(body)
		msg, _ := t.buildMessage(&event, meta, flatBody, body)
		t.enqueuePushTasks(asyncCtx, meta, &event, msg, flatBody)
	}()
}

func (t *EventTrigger) buildMessage(event *model.PushEvent, meta EventMetadata, flatBody map[string]any, body map[string]any) (NotificationMessage, string) {
	var msg NotificationMessage
	renderedTemplate := ""

	templateSource := event.Template
	if templateSource != "" {
		var err error
		msg, renderedTemplate, err = t.parseCustomTemplate(event, templateSource, flatBody)
		if err != nil {
			msg.Title = event.Name
			msg.Content = renderedTemplate
			msg.Level = defaultLevelInfo
		}
	} else {
		msg = t.parseDefaultTemplate(meta, flatBody)
	}

	if msg.Ext == nil {
		msg.Ext = make(map[string]any)
	}
	for k, v := range body {
		if k == keyTitle || k == keyContent || k == keyLevel {
			continue
		}
		if _, exists := msg.Ext[k]; !exists {
			msg.Ext[k] = v
		}
	}

	return msg, renderedTemplate
}

func (t *EventTrigger) parseCustomTemplate(event *model.PushEvent, templateSource string, flatBody map[string]any) (NotificationMessage, string, error) {
	var msg NotificationMessage
	renderedTemplate := pkgpush.ParseTemplate(templateSource, flatBody)

	var tMap map[string]any
	if err := json.Unmarshal([]byte(renderedTemplate), &tMap); err != nil {
		return msg, renderedTemplate, err
	}

	if title, ok := tMap[keyTitle].(string); ok && title != "" {
		msg.Title = title
	} else {
		msg.Title = event.Name
	}
	delete(tMap, keyTitle)

	if content, ok := tMap[keyContent].(string); ok && content != "" {
		msg.Content = content
	} else {
		msg.Content = renderedTemplate
	}
	delete(tMap, keyContent)

	if level, ok := tMap[keyLevel].(string); ok && level != "" {
		msg.Level = level
	} else {
		msg.Level = defaultLevelInfo
	}
	delete(tMap, keyLevel)

	msg.Ext = tMap
	return msg, renderedTemplate, nil
}

func (t *EventTrigger) parseDefaultTemplate(meta EventMetadata, flatBody map[string]any) NotificationMessage {
	var msg NotificationMessage
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

func (t *EventTrigger) enqueuePushTasks(ctx context.Context, meta EventMetadata, event *model.PushEvent, msg NotificationMessage, flatBody map[string]any) {
	for _, channelName := range event.Channels {
		customChannel, err := repository.GetActivePushChannelByName(ctx, channelName)
		if err == nil {
			t.enqueueCustomPushChannelTasks(ctx, meta, event, customChannel, msg, flatBody)
			continue
		}
		logger.WarnF(ctx, "push_event_trigger: channel %q not found in DB or disabled: %v", channelName, err)
	}
}

func (t *EventTrigger) enqueueCustomPushChannelTasks(ctx context.Context, meta EventMetadata, event *model.PushEvent, channel *model.PushChannel, msg NotificationMessage, flatBody map[string]any) {
	if len(event.Targets) == 0 {
		t.enqueueSingleCustomPushChannelTask(ctx, meta, channel, "", msg)
		return
	}

	for _, target := range event.Targets {
		resolvedTarget := resolveTarget(ctx, target, flatBody, channel.Name)
		t.enqueueSingleCustomPushChannelTask(ctx, meta, channel, resolvedTarget, msg)
	}
}

func (t *EventTrigger) enqueueSingleCustomPushChannelTask(ctx context.Context, meta EventMetadata, channel *model.PushChannel, target string, msg NotificationMessage) {
	var config pkgpush.Config
	var renderedTemplate string

	switch channel.Type {
	case channelLark:
		config = pkgpush.Config{Channel: channelLark, URL: channel.URL, Secret: channel.Token}
		renderedTemplate = channel.Other
	case channelEmail:
		url, token, other := resolveSMTPConfig(ctx, channel.URL, channel.Token, channel.Other)
		config = pkgpush.Config{Channel: channelEmail, URL: url, Key: token, Secret: other}
	case channelTelegram:
		config = pkgpush.Config{Channel: channelTelegram, URL: channel.URL, Secret: channel.Token, Key: channel.Other}
	default:
		config = pkgpush.Config{Channel: channelCustom, URL: channel.URL}
		customPushReq := CustomPushRequest{
			Title:       msg.Title,
			Content:     msg.Content,
			Description: meta.Description,
			To:          target,
		}
		if urlVal, ok := msg.Ext["url"].(string); ok {
			customPushReq.URL = urlVal
		}
		renderedTemplate = renderCustomPayload(channel.Other, customPushReq)
	}

	payload := SendPayload{
		EventKey: meta.Key,
		Config:   config,
		Target:   target,
		Body:     msg,
		Template: renderedTemplate,
	}
	if err := enqueuePushTask(ctx, payload); err != nil {
		logger.ErrorF(ctx, "push_event_trigger: enqueuePushTask failed for %s channel %s -> %s: %v", channel.Type, channel.Name, target, err)
	}
}

func enqueuePushTask(ctx context.Context, payload SendPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = task.DispatchTask(ctx, "send_notification", payloadBytes, "system")
	return err
}

func getFlatBody(body map[string]any) map[string]any {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return body
	}
	var jsonMap map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		return body
	}

	flatResult := make(map[string]any)
	flattenMap("", jsonMap, flatResult)
	return flatResult
}

func flattenMap(prefix string, m map[string]any, result map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nestedMap, ok := v.(map[string]any); ok {
			flattenMap(key, nestedMap, result)
		} else {
			result[key] = v
		}
	}
}

func resolveTarget(ctx context.Context, target string, flatBody map[string]any, channel string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	resolved := resolveDynamicKeyword(target, flatBody)
	if strings.Contains(resolved, "@") {
		return resolved
	}
	if val, matched := resolveSystemTarget(ctx, resolved, channel); matched {
		return val
	}

	user, found := resolveTargetUser(ctx, resolved, channel)
	if !found {
		return resolved
	}
	if channel == channelEmail && user.Email != "" {
		return user.Email
	}
	if channel != channelEmail && user.Username != "" {
		return user.Username
	}
	return resolved
}

func resolveDynamicKeyword(target string, flatBody map[string]any) string {
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
	case "user.email", channelEmail:
		if val, ok := flatBody["user.email"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["email"]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return target
}
