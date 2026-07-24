// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// StorageMigrationTask is the Asynq task name for storage migration.
const StorageMigrationTask = "storage:migrate"

// LatestMigrationExecution returns the most recent storage migration task execution.
func LatestMigrationExecution(ctx context.Context) (*model.TaskExecution, bool, error) {
	return repository.GetLatestTaskExecutionByTaskType(ctx, StorageMigrationTask)
}

// ParseMigrationTargetConfig parses and validates a storage migration target payload.
func ParseMigrationTargetConfig(ctx context.Context, payload []byte) (objectstore.Config, error) {
	if strings.TrimSpace(string(payload)) == "" {
		return objectstore.Config{}, errors.New("storage migration target payload is required")
	}

	var raw struct {
		Target json.RawMessage `json:"target"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return objectstore.Config{}, fmt.Errorf("parse storage migration payload envelope: %w", err)
	}

	if len(raw.Target) == 0 {
		return objectstore.Config{}, errors.New("storage migration target payload is required")
	}

	var targetBytes []byte
	var targetStr string
	if err := json.Unmarshal(raw.Target, &targetStr); err == nil {
		targetBytes = []byte(targetStr)
	} else {
		targetBytes = raw.Target
	}

	var target objectstore.Config
	if err := json.Unmarshal(targetBytes, &target); err != nil {
		return objectstore.Config{}, fmt.Errorf("parse target storage config: %w", err)
	}

	current, err := objectstore.LoadConfig(ctx)
	if err != nil {
		return objectstore.Config{}, fmt.Errorf("load active storage config: %w", err)
	}
	target = objectstore.MergeMaskedSecrets(target, current)
	if err := objectstore.ValidateConfig(target); err != nil {
		return objectstore.Config{}, fmt.Errorf("validate target storage config: %w", err)
	}
	return target, nil
}

// NormalizeMigrationPayload validates and normalizes a storage migration payload.
func NormalizeMigrationPayload(ctx context.Context, payload []byte) ([]byte, objectstore.Config, error) {
	target, err := ParseMigrationTargetConfig(ctx, payload)
	if err != nil {
		return nil, objectstore.Config{}, err
	}
	type storageMigrationPayload struct {
		Target objectstore.Config `json:"target"`
	}
	normalized, err := json.Marshal(storageMigrationPayload{Target: target})
	if err != nil {
		return nil, objectstore.Config{}, fmt.Errorf("marshal storage migration payload: %w", err)
	}
	return normalized, target, nil
}
