// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package qq implements the official QQ Bot C2C adapter.
package qq

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/service"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
	"golang.org/x/oauth2"
)

// qqEvent is a testable inbound envelope.
type qqEvent struct {
	Kind      string
	UserID    string
	Text      string
	MessageID string
}

// Adapter is an official QQ Bot C2C channel.
type Adapter struct {
	cfg          do.ChannelConfig
	onInbound    service.Handler
	api          openapi.OpenAPI
	tokenSrc     oauth2.TokenSource
	cancel       context.CancelFunc
	mu           sync.Mutex
	disconnected bool
}

// New constructs a QQ adapter.
func New(cfg do.ChannelConfig, onInbound service.Handler) (service.Channel, error) {
	if strings.TrimSpace(cfg.Credentials["app_id"]) == "" || strings.TrimSpace(cfg.Credentials["app_secret"]) == "" {
		return nil, fmt.Errorf("qq: app_id and app_secret are required")
	}
	return &Adapter{cfg: cfg, onInbound: onInbound}, nil
}

// Type returns qq.
func (a *Adapter) Type() string { return consts.ChannelTypeQQ }

// Capabilities reports C2C text/media support.
func (a *Adapter) Capabilities() do.Capability {
	return do.Capability{Text: true, Image: true, File: true, Reply: true}
}

// Connect starts the official WebSocket session (C2C intent).
func (a *Adapter) Connect(ctx context.Context) error {
	credentials := &token.QQBotCredentials{
		AppID:     a.cfg.Credentials["app_id"],
		AppSecret: a.cfg.Credentials["app_secret"],
	}
	tokSrc := token.NewQQBotTokenSource(credentials)
	runCtx, cancel := context.WithCancel(ctx)
	if err := token.StartRefreshAccessToken(runCtx, tokSrc); err != nil {
		cancel()
		return fmt.Errorf("qq: refresh token: %w", err)
	}

	var api openapi.OpenAPI
	const apiTimeout = 5 * time.Second
	if strings.EqualFold(strings.TrimSpace(a.cfg.Extra["sandbox"]), "true") {
		api = botgo.NewSandboxOpenAPI(credentials.AppID, tokSrc).WithTimeout(apiTimeout)
	} else {
		api = botgo.NewOpenAPI(credentials.AppID, tokSrc).WithTimeout(apiTimeout)
	}

	wsAP, err := api.WS(ctx, nil, "")
	if err != nil {
		cancel()
		return fmt.Errorf("qq: websocket ap: %w", err)
	}

	intent := event.RegisterHandlers(event.C2CMessageEventHandler(func(_ *dto.WSPayload, data *dto.WSC2CMessageData) error {
		authorID := ""
		if data != nil && data.Author != nil {
			authorID = data.Author.ID
		}
		text := ""
		id := ""
		if data != nil {
			text = data.Content
			id = data.ID
		}
		a.handleEvent(runCtx, qqEvent{Kind: "c2c", UserID: authorID, Text: text, MessageID: id})
		return nil
	}))

	a.mu.Lock()
	a.api = api
	a.tokenSrc = tokSrc
	a.cancel = cancel
	a.disconnected = false
	a.mu.Unlock()

	util.Go(func() {
		if err := botgo.NewSessionManager().Start(wsAP, tokSrc, &intent); err != nil {
			logger.ErrorF(runCtx, "qq session stopped: %v", err)
		}
	})
	return nil
}

// Disconnect stops token refresh and drops further inbound events.
func (a *Adapter) Disconnect(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.disconnected = true
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	return nil
}

// Send posts a C2C text reply.
func (a *Adapter) Send(ctx context.Context, to do.Recipient, msg do.OutboundMessage) error {
	a.mu.Lock()
	api := a.api
	a.mu.Unlock()
	if api == nil {
		return fmt.Errorf("qq: not connected")
	}
	_, err := api.PostC2CMessage(ctx, to.PlatformUserID, &dto.MessageToCreate{
		Content: msg.Text,
		MsgID:   msg.ReplyToID,
	})
	return err
}

func (a *Adapter) handleEvent(ctx context.Context, ev qqEvent) {
	if ev.Kind != "c2c" {
		return
	}
	a.mu.Lock()
	disconnected := a.disconnected
	a.mu.Unlock()
	if disconnected || a.onInbound == nil {
		return
	}
	_ = a.onInbound(ctx, do.InboundMessage{
		ChannelID:      a.cfg.ID,
		ChannelType:    consts.ChannelTypeQQ,
		PlatformUserID: ev.UserID,
		ChatID:         ev.UserID,
		MessageID:      ev.MessageID,
		Text:           ev.Text,
	})
}
