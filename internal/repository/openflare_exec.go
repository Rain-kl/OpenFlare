// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// CreateOpenFlareExecTask atomically persists a remote execution task and its initial per-node result placeholders.
func CreateOpenFlareExecTask(ctx context.Context, task *model.OpenFlareExecTask, initialResults []model.OpenFlareExecTaskResult) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		if len(initialResults) > 0 {
			if err := tx.Create(&initialResults).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveOpenFlareExecTaskResult updates a node's execution result for a task.
func SaveOpenFlareExecTaskResult(ctx context.Context, taskID, client, result string, exitCode int, finishedAt time.Time) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	res := conn.Model(&model.OpenFlareExecTaskResult{}).
		Where("task_id = ? AND client = ?", taskID, client).
		Updates(map[string]any{
			"result":      result,
			"exit_code":   exitCode,
			"finished_at": finishedAt.UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetOpenFlareExecTaskByID loads one task together with all of its per-node results.
func GetOpenFlareExecTaskByID(ctx context.Context, taskID string) (*model.OpenFlareExecTask, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var task model.OpenFlareExecTask
	if err := conn.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	var results []model.OpenFlareExecTaskResult
	if err := conn.Where("task_id = ?", taskID).Order("id ASC").Find(&results).Error; err != nil {
		return nil, err
	}
	task.Results = results
	return &task, nil
}

// ListOpenFlareExecTaskResults returns all per-node results for one task.
func ListOpenFlareExecTaskResults(ctx context.Context, taskID string) ([]model.OpenFlareExecTaskResult, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var results []model.OpenFlareExecTaskResult
	if err := conn.Where("task_id = ?", taskID).Order("id ASC").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// ListOpenFlareExecTasks returns recent remote execution tasks with attached per-node results.
func ListOpenFlareExecTasks(ctx context.Context, limit int) ([]*model.OpenFlareExecTask, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var tasks []*model.OpenFlareExecTask
	if err := conn.Order("created_at DESC").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return tasks, nil
	}
	taskIDs := make([]string, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.TaskID
	}
	var results []model.OpenFlareExecTaskResult
	if err := conn.Where("task_id IN ?", taskIDs).Order("id ASC").Find(&results).Error; err != nil {
		return nil, err
	}
	byTask := make(map[string][]model.OpenFlareExecTaskResult, len(tasks))
	for _, r := range results {
		byTask[r.TaskID] = append(byTask[r.TaskID], r)
	}
	for _, task := range tasks {
		if rs, ok := byTask[task.TaskID]; ok {
			task.Results = rs
		} else {
			task.Results = []model.OpenFlareExecTaskResult{}
		}
	}
	return tasks, nil
}

// DeleteOpenFlareExecTask removes a task and its per-node results.
func DeleteOpenFlareExecTask(ctx context.Context, taskID string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", taskID).Delete(&model.OpenFlareExecTaskResult{}).Error; err != nil {
			return err
		}
		res := tx.Where("task_id = ?", taskID).Delete(&model.OpenFlareExecTask{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// DeleteOpenFlareExecTasksBefore removes remote execution tasks and results older than cutoff.
func DeleteOpenFlareExecTasksBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	var deleted int64
	err := conn.Transaction(func(tx *gorm.DB) error {
		r1 := tx.Where("created_at < ?", cutoff.UTC()).Delete(&model.OpenFlareExecTaskResult{})
		if r1.Error != nil {
			return r1.Error
		}
		r2 := tx.Where("created_at < ?", cutoff.UTC()).Delete(&model.OpenFlareExecTask{})
		if r2.Error != nil {
			return r2.Error
		}
		deleted = r1.RowsAffected + r2.RowsAffected
		return nil
	})
	return deleted, err
}
