// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/Rain-kl/Wavelet/internal/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/router"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "wavelet API",
	Run: func(_ *cobra.Command, _ []string) {
		bootstrap.RegisterAPI()
		runBootstrap(bootstrap.Options{API: true})
		router.Serve()
	},
}
