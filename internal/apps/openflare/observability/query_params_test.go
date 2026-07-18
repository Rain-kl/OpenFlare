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
