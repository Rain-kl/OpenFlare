// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package references

import (
	"context"
	"errors"
	"fmt"

	"github.com/Rain-kl/Wavelet/pkg/logger"
	"go.uber.org/zap"
)

// CustomService 示例业务 Service 结构体（通常放在 internal/apps/custom/service.go 中）
type CustomService struct {
	// 这里可以注入数据库连接、配置对象或者其他基础服务的客户端
	// 例如：db *gorm.DB
}

// NewCustomService 创建 CustomService 实例的构造函数
func NewCustomService() *CustomService {
	return &CustomService{}
}

// ProcessBusinessData 演示核心业务处理逻辑的 Service 方法
// 1. 首位参数必须是 context.Context，以传播链路追踪 (OTel) 和超时控制。
// 2. 方法签名应该只包含纯 Go 的参数与返回值，禁止导入 Gin 或与 HTTP 相关的协议依赖。
// 3. 将可能发生的核心异常通过 error 返回给上层，而不是在这一层转换成 HTTP 状态码。
func (s *CustomService) ProcessBusinessData(ctx context.Context, userID int64, payload string) (string, error) {
	if payload == "" {
		return "", errors.New("payload cannot be empty")
	}

	// 模拟执行业务逻辑...
	logger.Info(ctx, "processing custom business data in service",
		zap.Int64("user_id", userID),
		zap.String("payload", payload),
	)

	// 这里可以包含数据库读写、事务控制、或者远程 API 调用等复杂逻辑。
	result := fmt.Sprintf("Success processed data for user %d: %s", userID, payload)

	return result, nil
}
