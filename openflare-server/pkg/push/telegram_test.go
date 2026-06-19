// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramPusher_Send(t *testing.T) {
	t.Run("successful send with HTML parse mode", func(t *testing.T) {
		var receivedReq telegramMessageRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/botmy-token/sendMessage", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			err := json.NewDecoder(r.Body).Decode(&receivedReq)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": true}`))
		}))
		defer server.Close()

		pusher := &TelegramPusher{}
		cfg := Config{
			Channel: "telegram",
			URL:     server.URL,
			Secret:  "my-token",
		}
		body := map[string]any{
			"title":   "Alert",
			"content": "Host down",
			"level":   "CRITICAL",
		}
		err := pusher.Send(context.Background(), cfg, "123456", body, "", nil)
		require.NoError(t, err)

		assert.Equal(t, "123456", receivedReq.ChatID)
		assert.Contains(t, receivedReq.Text, "[CRITICAL] Alert")
		assert.Contains(t, receivedReq.Text, "Host down")
		assert.Equal(t, "HTML", receivedReq.ParseMode)
	})

	t.Run("fallback to plain text on HTML error", func(t *testing.T) {
		var requests []*telegramMessageRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req telegramMessageRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			requests = append(requests, &req)

			if len(requests) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"ok": false, "error_code": 400, "description": "Bad Request: can't parse entities"}`))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok": true}`))
			}
		}))
		defer server.Close()

		pusher := &TelegramPusher{}
		cfg := Config{
			Channel: "telegram",
			URL:     server.URL,
			Secret:  "my-token",
		}
		body := map[string]any{
			"title":   "Alert & Info",
			"content": "A < B comparison",
			"level":   "INFO",
		}
		err := pusher.Send(context.Background(), cfg, "123456", body, "", nil)
		require.NoError(t, err)

		require.Len(t, requests, 2)
		assert.Equal(t, "HTML", requests[0].ParseMode)
		assert.Equal(t, "", requests[1].ParseMode)
		assert.Contains(t, requests[1].Text, "[INFO] Alert & Info")
		assert.Contains(t, requests[1].Text, "A < B comparison")
	})

	t.Run("validation error", func(t *testing.T) {
		pusher := &TelegramPusher{}
		cfg := Config{
			Channel: "telegram",
			URL:     "https://api.telegram.org",
		}
		err := pusher.ValidateConfig(cfg)
		assert.Error(t, err)

		cfg = Config{
			Channel: "telegram",
			URL:     "ftp://api.telegram.org",
			Secret:  "token",
		}
		err = pusher.ValidateConfig(cfg)
		assert.Error(t, err)

		cfg = Config{
			Channel: "telegram",
			Secret:  "token",
		}
		err = pusher.ValidateConfig(cfg)
		assert.NoError(t, err)
	})
}
