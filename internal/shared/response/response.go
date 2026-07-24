// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package response provides shared HTTP API response structures.
package response

import "github.com/gin-gonic/gin"

// Response 通用响应体
type Response[T any] struct {
	ErrorMsg string `json:"error_msg"`
	Data     T      `json:"data"`
}

// Any 用于 Swagger 文档的响应类型（非泛型）
// swag 不支持泛型，使用此类型替代 Response[T]
type Any struct {
	ErrorMsg string      `json:"error_msg" example:""`
	Data     interface{} `json:"data"`
}

// APIError 统一的 API 业务错误类型，可被全局错误处理中间件捕获
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return e.Msg
}

// NewError 实例化一个 APIError
func NewError(code int, msg string) *APIError {
	return &APIError{Code: code, Msg: msg}
}

// AbortWithError 将 API 错误挂载到 Gin Context 并中断执行流
func AbortWithError(c *gin.Context, code int, msg string) {
	_ = c.Error(NewError(code, msg))
	c.Abort()
}

// OK 构造成功响应
func OK[T any](data T) Response[T] {
	return Response[T]{Data: data}
}

// OKNil 构造成功响应（data 为 null）
func OKNil() Response[any] {
	return Response[any]{Data: nil}
}

// Err 构造错误响应
func Err(msg string) Response[any] {
	return Response[any]{ErrorMsg: msg, Data: nil}
}
