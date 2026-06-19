// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"github.com/Rain-kl/Wavelet/internal/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/task/worker"

	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "wavelet Worker",
	Run: func(_ *cobra.Command, _ []string) {
		runBootstrap(bootstrap.Options{})
		log.Println("[Worker] 启动任务处理服务")
		if err := worker.StartWorker(); err != nil {
			log.Fatalf("[工作器] 启动失败: %v", err)
		}
	},
}
