// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package telegram implements the Telegram private-chat adapter.
package telegram

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/service"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

// Adapter is a Telegram private-chat channel.
type Adapter struct {
	cfg       do.ChannelConfig
	onInbound service.Handler
	bot       *tele.Bot
}

// New constructs a Telegram adapter. Call service.Register from the runner.
func New(cfg do.ChannelConfig, onInbound service.Handler) (service.Channel, error) {
	if strings.TrimSpace(cfg.Credentials["bot_token"]) == "" {
		return nil, fmt.Errorf("telegram: bot_token is required")
	}
	return &Adapter{cfg: cfg, onInbound: onInbound}, nil
}

// Type returns telegram.
func (a *Adapter) Type() string { return consts.ChannelTypeTelegram }

// Capabilities reports private-chat media support.
func (a *Adapter) Capabilities() do.Capability {
	return do.Capability{Text: true, Image: true, File: true, Reply: true}
}

// longPollWindow is how long Telegram may hold a getUpdates call open before
// returning empty. telebot converts it with int(timeout / time.Second), so a
// bare integer here would mean nanoseconds, send timeout=0 and turn the poller
// into a tight loop against the Bot API.
const longPollWindow = 10 * time.Second

// buildTeleSettings assembles the telebot settings.
func buildTeleSettings(cfg do.ChannelConfig) tele.Settings {
	pref := tele.Settings{
		Token:  cfg.Credentials["bot_token"],
		Poller: &tele.LongPoller{Timeout: longPollWindow},
	}
	if base := strings.TrimSpace(cfg.Extra["base_url"]); base != "" {
		pref.URL = strings.TrimSuffix(base, "/")
	}
	return pref
}

// Connect starts long polling.
func (a *Adapter) Connect(ctx context.Context) error {
	bot, err := tele.NewBot(buildTeleSettings(a.cfg))
	if err != nil {
		return fmt.Errorf("telegram: new bot: %w", err)
	}
	a.bot = bot
	bot.Handle(tele.OnText, func(c tele.Context) error {
		a.handleTeleMessage(ctx, c.Message())
		return nil
	})
	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		a.handleTeleMessage(ctx, c.Message())
		return nil
	})
	bot.Handle(tele.OnDocument, func(c tele.Context) error {
		a.handleTeleMessage(ctx, c.Message())
		return nil
	})
	util.Go(func() {
		bot.Start()
	})
	util.Go(func() {
		<-ctx.Done()
		bot.Stop()
	})
	return nil
}

// Disconnect stops the bot.
func (a *Adapter) Disconnect(_ context.Context) error {
	if a.bot != nil {
		a.bot.Stop()
	}
	return nil
}

// Send replies to a private chat.
func (a *Adapter) Send(_ context.Context, to do.Recipient, msg do.OutboundMessage) error {
	if a.bot == nil {
		return fmt.Errorf("telegram: not connected")
	}
	chatID, err := strconv.ParseInt(to.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: chat id: %w", err)
	}
	_, err = a.bot.Send(tele.ChatID(chatID), msg.Text)
	return err
}

func (a *Adapter) handleTeleMessage(ctx context.Context, m *tele.Message) {
	if m == nil || m.Chat == nil || m.Chat.Type != tele.ChatPrivate {
		return
	}
	if a.onInbound == nil {
		return
	}
	msg := do.InboundMessage{
		ChannelID:      a.cfg.ID,
		ChannelType:    consts.ChannelTypeTelegram,
		PlatformUserID: strconv.FormatInt(m.Sender.ID, 10),
		ChatID:         strconv.FormatInt(m.Chat.ID, 10),
		MessageID:      strconv.Itoa(m.ID),
		Text:           m.Text,
	}
	if m.Caption != "" && msg.Text == "" {
		msg.Text = m.Caption
	}
	if a.bot != nil {
		dir, attachments := a.downloadMedia(m)
		if dir != "" {
			defer func() {
				if err := os.RemoveAll(dir); err != nil {
					logger.WarnF(ctx, "telegram: 清理临时媒体目录 %q 失败: %v", dir, err)
				}
			}()
		}
		msg.Attachments = attachments
	}
	_ = a.onInbound(ctx, msg)
}

// downloadMedia fetches message media into a scratch directory, returned so the
// caller can remove it once the inbound handler no longer needs the paths.
// An empty dir means nothing was downloaded.
func (a *Adapter) downloadMedia(m *tele.Message) (string, []do.Attachment) {
	var files []*tele.File
	var names []string
	if m.Photo != nil {
		files = append(files, m.Photo.MediaFile())
		names = append(names, "photo.jpg")
	}
	if m.Document != nil {
		files = append(files, &m.Document.File)
		name := m.Document.FileName
		if name == "" {
			name = "file"
		}
		names = append(names, name)
	}
	if len(files) == 0 {
		return "", nil
	}
	dir, err := os.MkdirTemp("", "wg-tg-*")
	if err != nil {
		return "", []do.Attachment{{Error: err.Error()}}
	}
	out := make([]do.Attachment, 0, len(files))
	for i, f := range files {
		path := filepath.Join(dir, names[i])
		if err := a.bot.Download(f, path); err != nil {
			out = append(out, do.Attachment{FileName: names[i], Error: err.Error()})
			continue
		}
		out = append(out, do.Attachment{Path: path, FileName: names[i]})
	}
	return dir, out
}
