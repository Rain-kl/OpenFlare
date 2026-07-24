// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
)

const (
	uploadMetaRedisCacheTTL    = 30 * 60 // seconds
	uploadMetaRAMMaximumSize   = 4096
	uploadMetaInvalidationChan = "upload:meta_invalidation"
)

type uploadMetaInvalidationMessage struct {
	ID uint64 `json:"id"`
}

var (
	uploadMetaRAM            = ram.MustNew[uint64, model.Upload](ram.Options{MaximumSize: uploadMetaRAMMaximumSize})
	uploadMetaListenerOnce   sync.Once
	uploadMetaListenerCtx    context.Context
	uploadMetaListenerCancel context.CancelFunc
)

func uploadMetaRedisKey(id uint64) string {
	return fmt.Sprintf("upload:meta:%d", id)
}

func cloneUpload(upload model.Upload) model.Upload {
	return upload
}

func ensureUploadMetaCacheListener() {
	if db.Redis == nil {
		return
	}
	uploadMetaListenerOnce.Do(startUploadMetaCacheInvalidationListener)
}

func startUploadMetaCacheInvalidationListener() {
	uploadMetaListenerCtx, uploadMetaListenerCancel = context.WithCancel(context.Background())

	go func() {
		pubsub := db.Redis.Subscribe(uploadMetaListenerCtx, uploadMetaInvalidationChan)
		defer func() {
			_ = pubsub.Close()
		}()

		go func() {
			<-uploadMetaListenerCtx.Done()
			_ = pubsub.Close()
		}()

		for msg := range pubsub.Channel() {
			var payload uploadMetaInvalidationMessage
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil || payload.ID == 0 {
				uploadMetaRAM.InvalidateAll()
				continue
			}
			uploadMetaRAM.Invalidate(payload.ID)
		}
	}()
}

func publishUploadMetaRAMInvalidation(ctx context.Context, id uint64) {
	if db.Redis == nil {
		return
	}
	payload, err := json.Marshal(uploadMetaInvalidationMessage{ID: id})
	if err != nil {
		return
	}
	_ = db.Redis.Publish(ctx, uploadMetaInvalidationChan, payload).Err()
}

// GetUploadByID loads upload metadata from RAM, Redis, or the database.
func GetUploadByID(ctx context.Context, id uint64) (model.Upload, error) {
	ensureUploadMetaCacheListener()

	if upload, ok := uploadMetaRAM.GetIfPresent(id); ok {
		return cloneUpload(upload), nil
	}

	key := uploadMetaRedisKey(id)
	if db.Redis != nil {
		var upload model.Upload
		if err := db.GetJSON(ctx, key, &upload); err == nil {
			uploadMetaRAM.Set(id, cloneUpload(upload))
			return upload, nil
		}
	}

	upload, err := repository.GetCacheableUploadByID(ctx, id)
	if err != nil {
		return model.Upload{}, err
	}

	SetUploadMetaCache(ctx, &upload)
	return upload, nil
}

// SetUploadMetaCache populates RAM and Redis upload metadata caches.
func SetUploadMetaCache(ctx context.Context, upload *model.Upload) {
	ensureUploadMetaCacheListener()

	if upload == nil {
		return
	}

	cloned := cloneUpload(*upload)
	uploadMetaRAM.Set(upload.ID, cloned)
	if db.Redis != nil {
		_ = db.SetJSON(ctx, uploadMetaRedisKey(upload.ID), cloned, uploadMetaRedisCacheTTL)
	}
}

// InvalidateUploadMetaCache clears RAM and Redis upload metadata caches and notifies peer nodes.
func InvalidateUploadMetaCache(ctx context.Context, id uint64) {
	ensureUploadMetaCacheListener()

	uploadMetaRAM.Invalidate(id)
	if db.Redis != nil {
		_ = db.Redis.Del(ctx, db.PrefixedKey(uploadMetaRedisKey(id))).Err()
		publishUploadMetaRAMInvalidation(ctx, id)
	}
}

// ResetUploadMetaCacheForTest clears the in-process upload metadata RAM cache.
func ResetUploadMetaCacheForTest() {
	uploadMetaRAM.InvalidateAll()
}

// StopUploadMetaCacheListener stops the Redis Pub/Sub subscription listener and resets the sync.Once guard.
func StopUploadMetaCacheListener() {
	if uploadMetaListenerCancel != nil {
		uploadMetaListenerCancel()
		uploadMetaListenerCancel = nil
	}
	uploadMetaListenerOnce = sync.Once{}
}
