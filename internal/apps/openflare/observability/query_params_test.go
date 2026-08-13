// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReadQueryStringArrayAcceptsHostsBracketForm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		url  string
		want []string
	}{
		{
			name: "axios brackets form",
			url:  "/overview?hours=168&hosts%5B%5D=gist.arctel.de",
			want: []string{"gist.arctel.de"},
		},
		{
			name: "repeated hosts keys",
			url:  "/overview?hosts=a.example&hosts=b.example",
			want: []string{"a.example", "b.example"},
		},
		{
			name: "single hosts key",
			url:  "/overview?hosts=gist.arctel.de",
			want: []string{"gist.arctel.de"},
		},
		{
			name: "empty",
			url:  "/overview?hours=24",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			c.Request = req

			got := readQueryStringArray(c, "hosts")
			require.Equal(t, tc.want, got)
		})
	}
}

func TestReadAccessLogQueryIncludesStatusCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(
		http.MethodGet,
		"/?node_id=n1&remote_addr=1.2.3.4&host=a.example&path=/api&status_code=404&p=2&page_size=50",
		nil,
	)
	require.NoError(t, err)
	c.Request = req

	got, err := readAccessLogQuery(c)
	require.NoError(t, err)
	require.Equal(t, "n1", got.NodeID)
	require.Equal(t, "1.2.3.4", got.RemoteAddr)
	require.Equal(t, "a.example", got.Host)
	require.Equal(t, "/api", got.Path)
	require.Equal(t, 404, got.StatusCode)
	require.Equal(t, 2, got.Page)
	require.Equal(t, 50, got.PageSize)
}

func TestReadAccessLogQueryRejectsInvalidStatusCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, raw := range []string{"abc", "99", "600", "-1"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, err := http.NewRequest(http.MethodGet, "/?status_code="+raw, nil)
		require.NoError(t, err)
		c.Request = req

		_, err = readAccessLogQuery(c)
		require.Error(t, err, "status_code=%s should be rejected", raw)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)
	c.Request = req
	got, err := readAccessLogQuery(c)
	require.NoError(t, err)
	require.Equal(t, 0, got.StatusCode)
}

func TestResolveAccessLogWindow(t *testing.T) {
	since, until, err := resolveAccessLogWindow(
		"2026-08-01T00:00:00Z",
		"2026-08-02T00:00:00Z",
	)
	require.NoError(t, err)
	require.True(t, until.After(since))

	for _, tc := range []struct {
		name  string
		since string
		until string
	}{
		{name: "missing both", since: "", until: ""},
		{name: "only since", since: "2026-08-01T00:00:00Z", until: ""},
		{name: "only until", since: "", until: "2026-08-02T00:00:00Z"},
		{name: "bad since", since: "not-a-time", until: "2026-08-02T00:00:00Z"},
		{name: "bad until", since: "2026-08-01T00:00:00Z", until: "not-a-time"},
		{name: "reversed", since: "2026-08-02T00:00:00Z", until: "2026-08-01T00:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := resolveAccessLogWindow(tc.since, tc.until)
			require.Error(t, err)
		})
	}
}
