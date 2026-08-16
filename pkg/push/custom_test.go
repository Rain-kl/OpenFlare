// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomPusherSend_ResponseBodyErrcode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "wechat business error returns HTTP 200 with non-zero errcode",
			statusCode: http.StatusOK,
			body:       `{"errcode":93000,"errmsg":"invalid request data"}`,
			wantErr:    true,
			wantErrMsg: "errcode=93000",
		},
		{
			name:       "wechat success returns errcode 0",
			statusCode: http.StatusOK,
			body:       `{"errcode":0,"errmsg":"ok"}`,
			wantErr:    false,
		},
		{
			name:       "json response without errcode is tolerated",
			statusCode: http.StatusOK,
			body:       `{"success":true}`,
			wantErr:    false,
		},
		{
			name:       "non-json response body is tolerated",
			statusCode: http.StatusOK,
			body:       "ok",
			wantErr:    false,
		},
		{
			name:       "empty response body is tolerated",
			statusCode: http.StatusNoContent,
			body:       "",
			wantErr:    false,
		},
		{
			name:       "http error status still fails",
			statusCode: http.StatusInternalServerError,
			body:       `{"errcode":0,"errmsg":"ok"}`,
			wantErr:    true,
			wantErrMsg: "http status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			pusher := &CustomPusher{}
			upstreamResp, err := pusher.Send(context.Background(),
				Config{Channel: "custom", URL: srv.URL},
				"",
				map[string]any{"title": "t", "content": "c"},
				`{"title":"$title","content":"$content"}`,
				nil,
			)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				return
			}
			assert.NoError(t, err)
			if tt.body != "" {
				assert.Contains(t, upstreamResp, tt.body)
			}
		})
	}
}
