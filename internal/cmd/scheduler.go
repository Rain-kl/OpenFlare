// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"github.com/Rain-kl/Wavelet/internal/infra/task/scheduler"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"

	"github.com/spf13/cobra"
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "wavelet Scheduler",
	Run: func(_ *cobra.Command, _ []string) {
		runBootstrap(bootstrap.Options{})
		printStartupBanner(startupState{mode: "Scheduler", relationalDB: latestMigrationState.relationalDB, clickHouseDB: latestMigrationState.clickHouseDB})
		log.Println("[Scheduler] Starting scheduled task service")
		if err := scheduler.StartScheduler(); err != nil {
			log.Fatalf("[Scheduler] Failed to start: %v", err)
		}
	},
}
