// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"slices"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"gorm.io/gorm"
)

// ListOpenFlarePingTasks returns all ping tasks ordered by weight ASC, id ASC.
func ListOpenFlarePingTasks(ctx context.Context) ([]*model.OpenFlarePingTask, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var tasks []*model.OpenFlarePingTask
	if err := conn.Order("weight ASC, id ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetOpenFlarePingTaskByID returns a single ping task by ID.
func GetOpenFlarePingTaskByID(ctx context.Context, id uint) (*model.OpenFlarePingTask, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var task model.OpenFlarePingTask
	if err := conn.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// CreateOpenFlarePingTask inserts a ping task and sets its initial weight to its generated ID (Komari behavior).
func CreateOpenFlarePingTask(ctx context.Context, task *model.OpenFlarePingTask) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		if task.Weight == 0 {
			task.Weight = int(task.ID)
			if err := tx.Model(task).Update("weight", task.Weight).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateOpenFlarePingTasks batch updates ping tasks in a single transaction.
func UpdateOpenFlarePingTasks(ctx context.Context, tasks []*model.OpenFlarePingTask) error {
	if len(tasks) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			res := tx.Model(&model.OpenFlarePingTask{}).Where("id = ?", task.ID).Updates(map[string]any{
				colName:       task.Name,
				colClients:    task.Clients,
				"all_clients": task.DefaultOn,
				colType:       task.Type,
				"target":      task.Target,
				"interval":    task.Interval,
				colUpdatedAt:  time.Now().UTC(),
			})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

// DeleteOpenFlarePingTasks deletes ping tasks by IDs and returns rows deleted.
func DeleteOpenFlarePingTasks(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	res := conn.Where("id IN ?", ids).Delete(&model.OpenFlarePingTask{})
	return res.RowsAffected, res.Error
}

// ReorderOpenFlarePingTasks validates all IDs exist and updates their weights atomically.
func ReorderOpenFlarePingTasks(ctx context.Context, weights map[uint]int) error {
	if len(weights) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	ids := make([]uint, 0, len(weights))
	for id := range weights {
		ids = append(ids, id)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&model.OpenFlarePingTask{}).Where("id IN ?", ids).Count(&existing).Error; err != nil {
			return err
		}
		if int(existing) != len(ids) {
			return gorm.ErrRecordNotFound
		}
		now := time.Now().UTC()
		for id, weight := range weights {
			if err := tx.Model(&model.OpenFlarePingTask{}).Where("id = ?", id).Updates(map[string]any{
				"weight":     weight,
				colUpdatedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// AttachNodeToDefaultOnPingTasks appends nodeID to all ping tasks with default_on=true (Komari AddDefaultOnClientUUID).
func AttachNodeToDefaultOnPingTasks(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		var tasks []model.OpenFlarePingTask
		if err := tx.Where("all_clients = ?", true).Find(&tasks).Error; err != nil {
			if isMissingTableError(err) {
				return nil
			}
			return err
		}
		now := time.Now().UTC()
		for _, task := range tasks {
			if slices.Contains(task.Clients, nodeID) {
				continue
			}
			task.Clients = append(task.Clients, nodeID)
			if err := tx.Model(&model.OpenFlarePingTask{}).Where("id = ?", task.ID).Updates(map[string]any{
				colClients:   task.Clients,
				colUpdatedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveNodeFromAllPingTasks removes a deleted nodeID from every ping task's client list.
func RemoveNodeFromAllPingTasks(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		var tasks []model.OpenFlarePingTask
		if err := tx.Find(&tasks).Error; err != nil {
			if isMissingTableError(err) {
				return nil
			}
			return err
		}
		now := time.Now().UTC()
		for _, task := range tasks {
			if !slices.Contains(task.Clients, nodeID) {
				continue
			}
			filtered := make(model.StringSliceJSON, 0, len(task.Clients))
			for _, id := range task.Clients {
				if id != nodeID {
					filtered = append(filtered, id)
				}
			}
			if err := tx.Model(&model.OpenFlarePingTask{}).Where("id = ?", task.ID).Updates(map[string]any{
				colClients:   filtered,
				colUpdatedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// InsertOpenFlarePingRecord enqueues a latency probe measurement into the active logstore.
func InsertOpenFlarePingRecord(ctx context.Context, record *model.OpenFlarePingRecord) error {
	s, err := logstore.Active(ctx)
	if err != nil {
		return err
	}
	return s.Observability.InsertPingRecord(ctx, record)
}

// ListOpenFlarePingRecords queries latency probe measurements from the active logstore.
func ListOpenFlarePingRecords(ctx context.Context, nodeID string, taskID uint, since time.Time, limit int) ([]*model.OpenFlarePingRecord, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.Observability.ListPingRecords(ctx, nodeID, taskID, since, limit)
}

// ListRecentOpenFlarePingRecordsByTask returns bounded samples for selected node-task pairs.
func ListRecentOpenFlarePingRecordsByTask(ctx context.Context, tasks map[uint]analyticsmodel.PingTaskWindow, limit int) ([]*model.OpenFlarePingRecord, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.Observability.ListRecentPingRecordsByTask(ctx, tasks, limit)
}

// DeleteOpenFlarePingRecordsByTaskIDs purges latency records belonging to deleted ping tasks.
func DeleteOpenFlarePingRecordsByTaskIDs(ctx context.Context, taskIDs []uint) (int64, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.Observability.DeletePingRecordsByTaskIDs(ctx, taskIDs)
}
