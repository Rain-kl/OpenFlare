// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"time"

	"github.com/Rain-kl/Wavelet/internal/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/hibiken/asynq"
)

// workerShutdownTimeout Worker 优雅关闭超时时间
const workerShutdownTimeout = 3 * time.Minute

// StartWorker 启动任务处理服务器
func StartWorker() error {
	bootstrap.RegisterWorker()
	asynqServer := asynq.NewServer(
		task.RedisOpt,
		asynq.Config{
			Concurrency:     config.Config.Worker.Concurrency,
			ShutdownTimeout: workerShutdownTimeout,
			Queues:          buildQueuesFromConfig(),
			StrictPriority:  config.Config.Worker.StrictPriority,
		},
	)

	// 注册 Asynq 任务路由
	mux := asynq.NewServeMux()
	mux.Use(taskLoggingMiddleware)

	// 统一使用 task.ProcessTask 处理所有任务类型
	// 框架内部自动分发到对应的 TaskHandler 实现
	// 动态注册所有已注册的任务处理器路由，框架内部自动分发到对应的 TaskHandler 实现
	for _, taskName := range task.GetRegisteredAsynqTasks() {
		mux.HandleFunc(taskName, task.ProcessTask)
	}

	// 启动服务器
	return asynqServer.Run(mux)
}

// buildQueuesFromConfig 从配置构建队列映射
func buildQueuesFromConfig() map[string]int {
	queues := make(map[string]int)

	// 从配置读取队列
	if len(config.Config.Worker.Queues) > 0 {
		for _, q := range config.Config.Worker.Queues {
			if q.Name != "" && q.Priority > 0 {
				queues[q.Name] = q.Priority
			}
		}
	}

	// 如果配置为空，使用默认队列
	if len(queues) == 0 {
		queues = map[string]int{
			task.QueueDefault: 1,
		}
	}

	return queues
}
