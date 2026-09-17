// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package testhelper

import (
	"bytes"
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/repository"

	"gorm.io/gorm"
)

// MockStorageService provides an in-memory contracts.StorageService for tests.
type MockStorageService struct {
	mu      sync.RWMutex
	objects map[string][]byte
	seq     uint64
}

// NewMockStorageService creates an initialized MockStorageService.
func NewMockStorageService() *MockStorageService {
	return &MockStorageService{
		objects: make(map[string][]byte),
	}
}

// Put writes an object into memory.
func (m *MockStorageService) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) (contracts.StoragePutResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(body)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}
	m.objects[key] = data
	return contracts.StoragePutResult{Key: key, Bucket: "test-bucket"}, nil
}

// Get reads an object from memory.
func (m *MockStorageService) Get(_ context.Context, key string) (*contracts.StorageObject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.objects[key]
	if ok {
		return &contracts.StorageObject{
			Key:           key,
			Body:          io.NopCloser(bytes.NewReader(data)),
			ContentLength: int64(len(data)),
			ContentType:   "application/octet-stream",
		}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// Delete removes an object from memory.
func (m *MockStorageService) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// Ingest ingests content into mock storage.
func (m *MockStorageService) Ingest(ctx context.Context, r io.Reader, opts contracts.IngestOptions) (*contracts.IngestResult, error) {
	id := atomic.AddUint64(&m.seq, 1)
	m.mu.Lock()
	data, _ := io.ReadAll(r)
	key := opts.FileName
	if key == "" {
		key = "file.dat"
	}
	m.objects[key] = data
	m.mu.Unlock()

	gdb := repository.DB(ctx)
	if gdb != nil {
		type testUpload struct {
			ID        uint64 `gorm:"primaryKey"`
			UserID    uint64
			FileName  string
			FilePath  string
			MimeType  string
			Size      int64
			Status    string
			Type      string
			Metadata  contracts.UploadMetadataDTO `gorm:"serializer:json;type:jsonb"`
			CreatedAt time.Time
			UpdatedAt time.Time
		}
		u := testUpload{
			ID:        id,
			UserID:    opts.UserID,
			FileName:  key,
			FilePath:  "mock/" + key,
			MimeType:  opts.MimeType,
			Size:      opts.Size,
			Status:    "used",
			Type:      opts.Type,
			Metadata:  contracts.UploadMetadataDTO{Extra: opts.Metadata},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		_ = gdb.Table("w_uploads").Save(&u).Error
	}
	return &contracts.IngestResult{ID: id, Key: "mock/" + key, Created: true, Stored: true}, nil
}
