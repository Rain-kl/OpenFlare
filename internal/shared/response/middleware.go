// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/pkg/i18n"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrorHandlerMiddleware 捕获 c.Errors 并统一格式化为 JSON 返回给客户端，同时将其记录到 Span 异常中。
// 与 AbortWithError / AbortBadRequest 等配合使用，是全局 OTel 友好错误响应的唯一出口。
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}

		err := c.Errors.Last().Err
		span := trace.SpanFromContext(c.Request.Context())
		if span.IsRecording() {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		var apiErr *APIError
		if errors.As(err, &apiErr) {
			c.JSON(apiErr.Code, Err(i18n.Translate(Locale(c), apiErr.Msg)))
			return
		}

		c.JSON(http.StatusInternalServerError, Err(i18n.Translate(Locale(c), "Internal system error")))
	}
}

// Locale uses the same cookie as the frontend, with Accept-Language for API clients.
func Locale(c *gin.Context) string {
	cookie, _ := c.Cookie("NEXT_LOCALE")
	return i18n.ResolveLocale(cookie, c.GetHeader("Accept-Language"))
}
