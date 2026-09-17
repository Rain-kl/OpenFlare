// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"crypto/rand"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// GenerateCode returns an 8-character pairing code using crypto/rand.
func GenerateCode() (string, error) {
	buf := make([]byte, consts.CodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, consts.CodeLength)
	for i, b := range buf {
		out[i] = consts.CodeAlphabet[int(b)%len(consts.CodeAlphabet)]
	}
	return string(out), nil
}

// NormalizeCode strips separators and uppercases.
func NormalizeCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

// FormatCode renders ABCD-EFGH format.
func FormatCode(s string) string {
	s = NormalizeCode(s)
	if len(s) != consts.CodeLength {
		return s
	}
	return s[:4] + "-" + s[4:]
}

// BindChannel consumes a pairing code and binds the platform identity to the user.
func BindChannel(ctx context.Context, userID uint64, req do.BindRequest) (do.BindingDTO, error) {
	channelID, err := strconv.ParseUint(strings.TrimSpace(req.ChannelID), 10, 64)
	if err != nil || channelID == 0 {
		return do.BindingDTO{}, consts.ErrChannelIDRequired
	}
	code := NormalizeCode(req.Code)
	if code == "" {
		return do.BindingDTO{}, consts.ErrCodeInvalid
	}
	pairing, err := dao.GetPairingCode(ctx, code)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return do.BindingDTO{}, consts.ErrCodeInvalid
		}
		return do.BindingDTO{}, err
	}
	if !pairing.ExpiresAt.After(time.Now()) {
		return do.BindingDTO{}, consts.ErrCodeInvalid
	}
	if pairing.ChannelID != channelID {
		return do.BindingDTO{}, consts.ErrChannelMismatch
	}
	ch, err := dao.GetMessageChannel(ctx, channelID)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return do.BindingDTO{}, consts.ErrCodeInvalid
		}
		return do.BindingDTO{}, err
	}
	if !ch.Enabled {
		return do.BindingDTO{}, consts.ErrChannelDisabled
	}

	existing, err := dao.GetBindingByChannelPlatform(ctx, channelID, pairing.PlatformUserID)
	if err != nil && !errors.Is(err, consts.ErrRecordNotFound) {
		return do.BindingDTO{}, err
	}
	if err == nil && existing != nil {
		if existing.UserID != userID {
			return do.BindingDTO{}, consts.ErrPlatformAlreadyBound
		}
		_ = dao.DeletePairingCode(ctx, pairing.Code)
		return ToBindingDTO(existing, ch), nil
	}

	row := &entity.MessageBinding{
		UserID:         userID,
		ChannelID:      channelID,
		PlatformUserID: pairing.PlatformUserID,
	}
	if err := dao.CreateMessageBinding(ctx, row); err != nil {
		return do.BindingDTO{}, err
	}
	if err := dao.DeletePairingCode(ctx, pairing.Code); err != nil {
		return do.BindingDTO{}, err
	}
	return ToBindingDTO(row, ch), nil
}

// ListEnabledPublicChannels returns the channels a user may bind to.
func ListEnabledPublicChannels(ctx context.Context) ([]do.PublicChannelDTO, error) {
	rows, err := dao.ListEnabledMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]do.PublicChannelDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, do.PublicChannelDTO{ID: row.ID, Name: row.Name, Type: row.Type})
	}
	return out, nil
}

// ListUserBindings returns the binding rows of one user enriched with channel info.
func ListUserBindings(ctx context.Context, userID uint64) ([]do.BindingDTO, error) {
	rows, err := dao.ListBindingsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]do.BindingDTO, 0, len(rows))
	for i := range rows {
		ch, err := dao.GetMessageChannel(ctx, rows[i].ChannelID)
		if err != nil {
			out = append(out, ToBindingDTO(&rows[i], nil))
			continue
		}
		out = append(out, ToBindingDTO(&rows[i], ch))
	}
	return out, nil
}

// UnbindChannel removes a binding owned by the given user.
func UnbindChannel(ctx context.Context, userID, bindingID uint64) error {
	row, err := dao.GetMessageBinding(ctx, bindingID)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return consts.ErrBindingNotFound
		}
		return err
	}
	if row.UserID != userID {
		return consts.ErrBindingForbidden
	}
	return dao.DeleteMessageBinding(ctx, bindingID)
}

// ToBindingDTO projects a binding row and its optional channel onto the user DTO.
func ToBindingDTO(row *entity.MessageBinding, ch *entity.MessageChannel) do.BindingDTO {
	dto := do.BindingDTO{
		ID:             row.ID,
		UserID:         row.UserID,
		ChannelID:      row.ChannelID,
		PlatformUserID: row.PlatformUserID,
		CreatedAt:      row.CreatedAt,
	}
	if ch != nil {
		dto.ChannelName = ch.Name
		dto.ChannelType = ch.Type
	}
	return dto
}
