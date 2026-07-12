// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	"github.com/Rain-kl/Wavelet/internal/db/migrator"
	"github.com/spf13/cobra"
)

var migrateZonesCmd = &cobra.Command{
	Use: "migrate-zones", Short: "导入旧域名数据到 Zone",
	PreRun: func(_ *cobra.Command, _ []string) { migrator.Migrate() },
	RunE: func(_ *cobra.Command, _ []string) error {
		report, err := zone.ImportLegacy(context.Background())
		return report.LogAndReturn(err)
	},
}
