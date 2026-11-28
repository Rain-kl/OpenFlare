// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestContext(method, target string, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Params = params
	return c, w
}

func TestParseUintHelpers(t *testing.T) {
	v32, err := ParseUint32("4294967295")
	require.NoError(t, err)
	require.Equal(t, uint32(math.MaxUint32), v32)

	for _, invalid := range []string{"", "-1", "4294967296", "4294971430", "abc"} {
		_, parseErr := ParseUint32(invalid)
		require.Error(t, parseErr, "expected ParseUint32(%q) to fail", invalid)
	}

	v64, err := ParseUint64("18446744073709551615")
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), v64)

	for _, invalid := range []string{"", "-1", "18446744073709551616", "abc"} {
		_, parseErr := ParseUint64(invalid)
		require.Error(t, parseErr, "expected ParseUint64(%q) to fail", invalid)
	}

	vu, err := ParseUint("42")
	require.NoError(t, err)
	require.Equal(t, uint(42), vu)

	overflowUint := strconv.FormatUint(uint64(^uint(0)), 10) + "0"
	_, err = ParseUint(overflowUint)
	require.Error(t, err)
}

func TestOptionalPositiveUint32Query(t *testing.T) {
	t.Run("omitted returns zero", func(t *testing.T) {
		c, w := newTestContext(http.MethodGet, "/test", nil)
		val, ok := OptionalPositiveUint32Query(c, "target_asn", "invalid asn")
		require.True(t, ok)
		require.Equal(t, uint32(0), val)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("empty returns zero", func(t *testing.T) {
		c, _ := newTestContext(http.MethodGet, "/test?target_asn=", nil)
		val, ok := OptionalPositiveUint32Query(c, "target_asn", "invalid asn")
		require.True(t, ok)
		require.Equal(t, uint32(0), val)
	})

	t.Run("valid uint32 boundaries", func(t *testing.T) {
		for _, tc := range []struct {
			raw  string
			want uint32
		}{
			{raw: "4134", want: 4134},
			{raw: "%204134%20", want: 4134},
			{raw: "4294967295", want: math.MaxUint32},
		} {
			c, _ := newTestContext(http.MethodGet, "/test?target_asn="+tc.raw, nil)
			val, ok := OptionalPositiveUint32Query(c, "target_asn", "invalid asn")
			require.True(t, ok)
			require.Equal(t, tc.want, val)
		}
	})

	t.Run("invalid or overflowing values abort with 400", func(t *testing.T) {
		r := gin.New()
		r.Use(response.ErrorHandlerMiddleware())
		r.GET("/test", func(c *gin.Context) {
			val, ok := OptionalPositiveUint32Query(c, "target_asn", "valid non-zero ASN is required")
			if !ok {
				return
			}
			c.JSON(http.StatusOK, response.OK(val))
		})

		for _, raw := range []string{"0", "-1", "4294967296", "4294967297", "4294971430", "abc", "%20"} {
			req := httptest.NewRequest(http.MethodGet, "/test?target_asn="+raw, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for target_asn=%s", raw)
		}
	})
}

func TestPositiveParamHelpers(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/asn/4294967295", gin.Params{{Key: "asn", Value: "4294967295"}})
	asn, ok := PositiveUint32Param(c, "asn", "invalid asn")
	require.True(t, ok)
	require.Equal(t, uint32(math.MaxUint32), asn)

	cBad, _ := newTestContext(http.MethodGet, "/asn/4294967296", gin.Params{{Key: "asn", Value: "4294967296"}})
	_, ok = PositiveUint32Param(cBad, "asn", "invalid asn")
	require.False(t, ok)
	require.True(t, cBad.IsAborted())

	c64, _ := newTestContext(http.MethodGet, "/users/18446744073709551615", gin.Params{{Key: "id", Value: "18446744073709551615"}})
	uid, ok := PositiveUint64Param(c64, "id", "invalid user")
	require.True(t, ok)
	require.Equal(t, uint64(math.MaxUint64), uid)

	c64Zero, _ := newTestContext(http.MethodGet, "/users/0", gin.Params{{Key: "id", Value: "0"}})
	_, ok = PositiveUint64Param(c64Zero, "id", "invalid user")
	require.False(t, ok)
	require.True(t, c64Zero.IsAborted())
}
