// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const pingoraEngineName = "pingora"

// EnsureActivePingoraRelease publishes the active site data in the current
// format during controller upgrades, without carrying over old tuning options.
func EnsureActivePingoraRelease(ctx context.Context) error {
	active, err := repository.GetActivePingoraConfigVersion(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		active, err = repository.GetActiveConfigVersionByEngine(ctx, "openresty")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	source, err := repository.GetConfigVersionSource(ctx, active)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source.SnapshotJSON), &fields); err != nil {
		return err
	}
	if active.Engine == pingoraEngineName && active.PayloadChecksum != "" && fields["edge_config"] != nil {
		return nil
	}

	var document edgeconfig.Document
	if err := json.Unmarshal([]byte(source.SnapshotJSON), &document); err != nil {
		return err
	}
	if fields == nil {
		return errors.New(errInvalidSnapshotFormat)
	}
	delete(fields, "openresty_config")
	fields["edge_config"], err = json.Marshal(document.Settings)
	if err != nil {
		return err
	}
	snapshot, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var existing []SupportFile
	if err := json.Unmarshal([]byte(source.SupportFilesJSON), &existing); err != nil {
		return err
	}
	certificates := make([]SupportFile, 0, len(existing))
	for _, file := range existing {
		if strings.HasSuffix(file.Path, ".crt") || strings.HasSuffix(file.Path, ".key") {
			certificates = append(certificates, file)
		}
	}
	bundle, err := renderSnapshotConfig(string(snapshot), certificates)
	if err != nil {
		return err
	}
	supportFiles, err := json.Marshal(bundle.SupportFiles)
	if err != nil {
		return err
	}
	return repository.EnsureActivePingoraRelease(ctx, active, &model.ConfigVersion{
		SnapshotJSON:     string(snapshot),
		MainConfig:       bundle.MainConfig,
		RenderedConfig:   bundle.RouteConfig,
		SupportFilesJSON: string(supportFiles),
		Checksum:         bundle.Checksum,
		IsActive:         true,
		Engine:           pingoraEngineName,
		PayloadChecksum:  bundle.Checksum,
		CreatedBy:        source.CreatedBy,
	})
}
