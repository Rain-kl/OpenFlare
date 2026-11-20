// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"gorm.io/gorm"
)

func getActiveConfigMetaByEngine(ctx context.Context, engine string) (*protocol.ActiveConfigMeta, error) {
	version, err := repository.GetActiveConfigVersionByEngine(ctx, engine)
	if err != nil {
		return nil, err
	}
	return &protocol.ActiveConfigMeta{
		Version:         version.Version,
		Checksum:        version.Checksum,
		Engine:          version.Engine,
		ReleaseSequence: version.ReleaseSequence,
		PayloadChecksum: version.PayloadChecksum,
	}, nil
}

// GetActivePingoraConfig returns the dedicated active Pingora configuration.
func GetActivePingoraConfig(ctx context.Context) (*PingoraConfigResponse, error) {
	version, err := repository.GetActivePingoraConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version.PayloadChecksum == "" {
		return nil, errors.New(errNoActiveConfig)
	}
	source, err := repository.GetConfigVersionSource(ctx, version)
	if err != nil {
		return nil, err
	}
	payload, err := edgeconfig.Encode(source.SnapshotJSON, source.SupportFilesJSON)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(payload))
	if hex.EncodeToString(digest[:]) != version.PayloadChecksum {
		return nil, errors.New(errInvalidEdgeSnapshotDigest)
	}
	return &PingoraConfigResponse{
		Version:         version.Version,
		Checksum:        version.Checksum,
		Engine:          version.Engine,
		ReleaseSequence: version.ReleaseSequence,
		PayloadJSON:     payload,
		PayloadChecksum: version.PayloadChecksum,
		CreatedAt:       version.CreatedAt,
	}, nil
}

func pingoraPagesProjectAllowed(ctx context.Context, projectID uint) (bool, error) {
	release, err := repository.GetActivePingoraConfigVersion(ctx)
	if isActiveConfigNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	source, err := repository.GetConfigVersionSource(ctx, release)
	if err != nil {
		return false, err
	}
	return edgeconfig.ContainsPagesProject(source.SnapshotJSON, projectID)
}

func isActiveConfigNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
