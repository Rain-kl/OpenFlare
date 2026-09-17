// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_cron

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type inprocScheduler struct {
	mu          sync.RWMutex
	cronRunner  *cron.Cron
	scheduleReg extpoints.ScheduleExtension
	taskReg     extpoints.TaskExtension
	taskSvc     contracts.TaskService
	running     bool
}

func newInprocScheduler(scheduleReg extpoints.ScheduleExtension, taskReg extpoints.TaskExtension, taskSvc contracts.TaskService) *inprocScheduler {
	return &inprocScheduler{
		cronRunner:  cron.New(cron.WithSeconds()),
		scheduleReg: scheduleReg,
		taskReg:     taskReg,
		taskSvc:     taskSvc,
	}
}

func (s *inprocScheduler) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if s.scheduleReg != nil {
		for _, def := range s.scheduleReg.Schedules() {
			s.registerJob(ctx, def)
		}
	}

	s.cronRunner.Start()
	s.running = true
	return nil
}

func (s *inprocScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cronRunner.Stop()
	<-ctx.Done()
	s.running = false
}

const standardCronFields = 5

func (s *inprocScheduler) registerJob(ctx context.Context, def extpoints.ScheduleDefinition) {
	spec := def.Spec
	taskType := def.TaskType

	fields := len(strings.Fields(spec))
	cronSpec := spec
	if fields == standardCronFields {
		cronSpec = "0 " + spec
	}

	var payloadBytes []byte
	if def.Payload != nil {
		switch p := def.Payload.(type) {
		case []byte:
			payloadBytes = p
		case string:
			payloadBytes = []byte(p)
		default:
			payloadBytes, _ = json.Marshal(p)
		}
	}

	_, err := s.cronRunner.AddFunc(cronSpec, func() {
		if s.taskSvc != nil {
			if _, dispatchErr := s.taskSvc.Dispatch(ctx, taskType, payloadBytes, contracts.TaskTriggerSchedule); dispatchErr != nil {
				logger.ErrorF(ctx, "driver_inproc_cron: dispatch task %q failed: %v", taskType, dispatchErr)
			}
			return
		}

		if s.taskReg != nil {
			if td, ok := s.taskReg.Get(taskType); ok {
				util.Go(func() {
					timeout := td.Timeout
					if timeout <= 0 {
						timeout = 5 * time.Minute
					}
					runCtx, cancel := context.WithTimeout(ctx, timeout)
					defer cancel()
					_ = invokeHandler(runCtx, td.Handler, payloadBytes)
				})
			}
		}
	})
	if err != nil {
		logger.ErrorF(ctx, "driver_inproc_cron: invalid cron spec %q for task %q: %v", spec, taskType, err)
	}
}

func invokeHandler(ctx context.Context, handler any, payload []byte) error {
	if handler == nil {
		return errors.New("nil task handler")
	}

	switch fn := handler.(type) {
	case func(context.Context, []byte) error:
		return fn(ctx, payload)
	case func(context.Context) error:
		return fn(ctx)
	case func([]byte) error:
		return fn(payload)
	case func() error:
		return fn()
	default:
		return fmt.Errorf("unsupported handler type: %T", handler)
	}
}
