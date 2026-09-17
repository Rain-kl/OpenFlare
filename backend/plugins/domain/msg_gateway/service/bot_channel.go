// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tencent-connect/botgo/token"
)

const defaultTelegramAPI = "https://api.telegram.org"

// ListDefinitions returns the admin form schema of every supported channel type.
func ListDefinitions() []do.Definition {
	return []do.Definition{
		{
			Type: consts.MessageChannelTypeTelegram,
			Fields: []do.Field{
				{Key: "token", Type: consts.TypePassword, Required: true},
				{Key: "api_base", Type: consts.TypeText, Required: false},
			},
		},
		{
			Type: consts.MessageChannelTypeQQ,
			Fields: []do.Field{
				{Key: "app_id", Type: consts.TypeText, Required: true},
				{Key: "client_secret", Type: "password", Required: true},
			},
		},
	}
}

// CreateChannel validates the admin payload and persists an encrypted channel.
func CreateChannel(ctx context.Context, req do.CreateChannelRequest) (do.ChannelDTO, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return do.ChannelDTO{}, errors.New(consts.ErrNameRequired)
	}
	channelType := strings.TrimSpace(req.Type)
	if channelType != consts.MessageChannelTypeTelegram && channelType != consts.MessageChannelTypeQQ {
		return do.ChannelDTO{}, errors.New(consts.ErrTypeInvalid)
	}
	creds := req.Credentials
	if creds == nil {
		creds = map[string]string{}
	}
	if err := ValidateCredentials(channelType, creds, false); err != nil {
		return do.ChannelDTO{}, err
	}
	cipher, err := EncryptCredentials(creds)
	if err != nil {
		return do.ChannelDTO{}, err
	}
	extra := req.Extra
	if extra == nil {
		extra = map[string]string{}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := &entity.MessageChannel{
		Name:        name,
		Type:        channelType,
		OwnerScope:  consts.MessageOwnerScopeSystem,
		Enabled:     enabled,
		Credentials: cipher,
		Extra:       EncodeExtra(extra),
	}
	if err := dao.CreateMessageChannel(ctx, row); err != nil {
		return do.ChannelDTO{}, err
	}
	return ToDTO(row, creds, extra), nil
}

// UpdateChannel patches a channel; empty secrets keep the stored ciphertext.
func UpdateChannel(ctx context.Context, id uint64, req do.UpdateChannelRequest) (do.ChannelDTO, error) {
	row, err := dao.GetMessageChannel(ctx, id)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return do.ChannelDTO{}, errors.New(consts.ErrChannelNotFoundText)
		}
		return do.ChannelDTO{}, err
	}
	creds, err := DecryptCredentials(row.Credentials)
	if err != nil {
		return do.ChannelDTO{}, err
	}
	extra := ParseExtra(row.Extra)

	if name := strings.TrimSpace(req.Name); name != "" {
		row.Name = name
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if req.Extra != nil {
		extra = req.Extra
	}
	if len(req.Credentials) > 0 {
		merged := make(map[string]string, len(creds))
		for k, v := range creds {
			merged[k] = v
		}
		for k, v := range req.Credentials {
			if strings.TrimSpace(v) == "" {
				continue
			}
			merged[k] = v
		}
		if err := ValidateCredentials(row.Type, merged, true); err != nil {
			return do.ChannelDTO{}, err
		}
		creds = merged
	}

	cipher, err := EncryptCredentials(creds)
	if err != nil {
		return do.ChannelDTO{}, err
	}
	row.Credentials = cipher
	row.Extra = EncodeExtra(extra)
	if err := dao.UpdateMessageChannel(ctx, row); err != nil {
		return do.ChannelDTO{}, err
	}
	return ToDTO(row, creds, extra), nil
}

// ListChannels returns every channel with secrets masked.
func ListChannels(ctx context.Context) ([]do.ChannelDTO, error) {
	rows, err := dao.ListMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]do.ChannelDTO, 0, len(rows))
	for i := range rows {
		creds, _ := DecryptCredentials(rows[i].Credentials)
		extra := ParseExtra(rows[i].Extra)
		out = append(out, ToDTO(&rows[i], creds, extra))
	}
	return out, nil
}

// DeleteChannel removes a channel together with its bindings and pairing codes.
func DeleteChannel(ctx context.Context, id uint64) error {
	if _, err := dao.GetMessageChannel(ctx, id); err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return errors.New(consts.ErrChannelNotFoundText)
		}
		return err
	}
	return dao.DeleteMessageChannel(ctx, id)
}

// ProbeChannel verifies the stored credentials against the upstream platform.
func ProbeChannel(ctx context.Context, id uint64) error {
	row, err := dao.GetMessageChannel(ctx, id)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return errors.New(consts.ErrChannelNotFoundText)
		}
		return err
	}
	creds, err := DecryptCredentials(row.Credentials)
	if err != nil {
		return err
	}
	switch row.Type {
	case consts.MessageChannelTypeTelegram:
		return ProbeTelegram(ctx, creds)
	case consts.MessageChannelTypeQQ:
		return ProbeQQ(ctx, creds)
	default:
		return errors.New(consts.ErrTypeInvalid)
	}
}

// ProbeTelegram calls getMe to confirm the bot token is usable.
func ProbeTelegram(ctx context.Context, creds map[string]string) error {
	tok := creds["token"]
	if strings.TrimSpace(tok) == "" {
		return errors.New(consts.ErrMissingTelegramToken)
	}
	base := creds["api_base"]
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = defaultTelegramAPI
	}
	url := fmt.Sprintf("%s/bot%s/getMe", base, tok)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s (%d): %s", consts.ErrTelegramGetMeFailed, resp.StatusCode, string(body))
	}
	var res struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	if !res.OK {
		return fmt.Errorf("%s: %s", consts.ErrTelegramNotOK, string(body))
	}
	return nil
}

// ProbeQQ exchanges the app credentials for an access token.
func ProbeQQ(_ context.Context, creds map[string]string) error {
	appID := strings.TrimSpace(creds["app_id"])
	secret := strings.TrimSpace(creds["client_secret"])
	if appID == "" || secret == "" {
		return errors.New(consts.ErrMissingQQCredentials)
	}
	credentials := &token.QQBotCredentials{
		AppID:     appID,
		AppSecret: secret,
	}
	tokSrc := token.NewQQBotTokenSource(credentials)
	tok, err := tokSrc.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", consts.ErrQQTokenFetchFailed, err)
	}
	if tok == nil || tok.AccessToken == "" {
		return errors.New(consts.ErrQQEmptyToken)
	}
	return nil
}

// ValidateCredentials checks the admin submitted credentials for a channel type.
func ValidateCredentials(t string, creds map[string]string, isUpdate bool) error {
	switch t {
	case consts.MessageChannelTypeTelegram:
		tok := creds["token"]
		if strings.TrimSpace(tok) == "" && !isUpdate {
			return errors.New(consts.ErrTelegramTokenRequired)
		}
		if base, ok := creds["api_base"]; ok && strings.TrimSpace(base) != "" {
			if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
				return errors.New(consts.ErrAPIBaseInvalid)
			}
		}
	case consts.MessageChannelTypeQQ:
		appID := creds["app_id"]
		secret := creds["client_secret"]
		if (strings.TrimSpace(appID) == "" || strings.TrimSpace(secret) == "") && !isUpdate {
			return errors.New(consts.ErrQQCredentialsRequired)
		}
	default:
		return errors.New(consts.ErrTypeInvalid)
	}
	return nil
}

// ToDTO projects a channel row onto the admin DTO with credentials masked.
func ToDTO(row *entity.MessageChannel, creds, extra map[string]string) do.ChannelDTO {
	return do.ChannelDTO{
		ID:          row.ID,
		Name:        row.Name,
		Type:        row.Type,
		OwnerScope:  row.OwnerScope,
		OwnerID:     row.OwnerID,
		Enabled:     row.Enabled,
		Credentials: MaskCredentials(row.Type, creds),
		Extra:       extra,
	}
}
