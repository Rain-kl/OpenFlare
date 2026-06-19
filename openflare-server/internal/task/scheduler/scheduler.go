// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Rain-kl/Wavelet/internal/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/pkg/logger"

	"github.com/hibiken/asynq"
)

var (
	activeScheduler *asynq.Scheduler
	schedulerMutex  sync.Mutex
	quitChan        chan struct{}
	schedulerOnce   sync.Once
)

// GetAsynqClient 获取全局 AsynqClient
func GetAsynqClient() *asynq.Client {
	return task.AsynqClient
}

// StartScheduler 启动调度器 (该函数阻塞，直到调度器退出)
func StartScheduler() error {
	bootstrap.RegisterScheduler()

	var err error
	schedulerOnce.Do(func() {
		quitChan = make(chan struct{})
		done := quitChan

		// 初始化并运行首次调度
		if err = ReloadScheduler(); err != nil {
			err = fmt.Errorf("initial reload failed: %w", err)
			return
		}

		signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stopSignals()

		if waitForStop(done, signalCtx.Done()) {
			StopScheduler()
		}
	})
	return err
}

// StopScheduler 停止调度服务并解除 StartScheduler 阻塞
func StopScheduler() {
	schedulerMutex.Lock()
	defer schedulerMutex.Unlock()

	if activeScheduler != nil {
		activeScheduler.Shutdown()
		activeScheduler = nil
	}

	if quitChan != nil {
		close(quitChan)
		quitChan = nil
	}
}

// ReloadScheduler 重载调度器配置 (线程安全)
func ReloadScheduler() error {
	schedulerMutex.Lock()
	defer schedulerMutex.Unlock()

	// 1. 如果有运行中的调度器，先关闭它
	if activeScheduler != nil {
		activeScheduler.Shutdown()
		activeScheduler = nil
	}

	// 2. 从数据库载入启用的定时任务配置
	schedules, err := model.ListActiveSchedules(context.Background())
	if err != nil {
		return fmt.Errorf("load schedules from db failed: %w", err)
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return fmt.Errorf(errLoadLocationFailed, err)
	}

	// 3. 实例化新的调度器
	newScheduler := asynq.NewScheduler(
		task.RedisOpt,
		&asynq.SchedulerOpts{
			Location: location,
		},
	)

	// 4. 遍历并注册任务
	for _, s := range schedules {
		meta := task.GetTaskMeta(s.TaskType)
		if meta == nil {
			continue // 忽略排程配置中无效的任务类型
		}

		// 构造 Asynq 载荷。定时任务使用对应 Meta 中的 Asynq 标识，同时将数据库中保存的 json 作为参数
		t := asynq.NewTask(meta.AsynqTask, []byte(s.Payload))

		if _, err := newScheduler.Register(
			s.Cron,
			t,
			asynq.MaxRetry(meta.MaxRetry),
			asynq.Queue(meta.Queue),
		); err != nil {
			// 定时任务配置可能有误（如 Cron 格式不被 Asynq 识别），记录日志并跳过
			logger.ErrorF(context.Background(), "[Scheduler] 注册定时任务失败 id=%d name=%s: %v", s.ID, s.Name, err)
			continue
		}
	}

	// 5. 启动并替换全局调度器。进程信号由 StartScheduler 统一处理。
	if err := newScheduler.Start(); err != nil {
		return fmt.Errorf("start scheduler failed: %w", err)
	}
	activeScheduler = newScheduler

	logger.InfoF(context.Background(), "[Scheduler] 成功重新加载定时任务，共注册 %d 个活动任务", len(schedules))
	return nil
}

func waitForStop(done <-chan struct{}, signals <-chan struct{}) bool {
	select {
	case <-done:
		return false
	case <-signals:
		return true
	}
}
