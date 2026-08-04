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

// ProcessLocalBusiness 示例的模块内部闭环业务逻辑
// 1. 存放在 apps/custom/logics.go 下，遵循纯 Go 规范，不强依赖 gin.Context，以便逻辑清晰和便于单元测试。
// 2. 用于当前应用模块内的简单业务或通用过程。
func ProcessLocalBusiness(ctx context.Context, userID int64, param string) (string, error) {
	if param == "" {
		return "", errors.New("param cannot be empty")
	}

	logger.Info(ctx, "processing local business inside apps/custom/logics",
		zap.Int64("user_id", userID),
		zap.String("param", param),
	)

	// 执行轻量级、无需跨模块/多入口复用的本地计算或模型操作
	result := fmt.Sprintf("Processed local logic for user %d: %s", userID, param)

	return result, nil
}
