// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"github.com/Rain-kl/Wavelet/internal/infra/task/worker"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"

	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "wavelet Worker",
	Run: func(_ *cobra.Command, _ []string) {
		runBootstrap(bootstrap.Options{})
		printStartupBanner(startupState{mode: "Worker", relationalDB: latestMigrationState.relationalDB, clickHouseDB: latestMigrationState.clickHouseDB})
		log.Println("[Worker] Starting task processing service")
		if err := worker.StartWorker(); err != nil {
			log.Fatalf("[Worker] Failed to start: %v", err)
		}
	},
}
