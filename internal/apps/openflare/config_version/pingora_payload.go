// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	pingoraEngineName    = "pingora"
	pingoraVersionPrefix = "pingora-"
)

func buildPingoraConfigVersion(ctx context.Context, source *model.ConfigVersion) (*model.ConfigVersion, error) {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyPingoraEnabled)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil || !enabled {
		return nil, err
	}
	payload, err := edgeconfig.Encode(source.SnapshotJSON, source.SupportFilesJSON)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(payload))
	return &model.ConfigVersion{
		Version:         pingoraVersionPrefix + source.Version,
		SourceVersion:   source.Version,
		Checksum:        source.Checksum,
		IsActive:        true,
		Engine:          pingoraEngineName,
		PayloadChecksum: hex.EncodeToString(digest[:]),
		CreatedBy:       source.CreatedBy,
	}, nil
}
