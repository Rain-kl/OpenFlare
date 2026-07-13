// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/migrator"
)

func TestFormatStartupBanner(t *testing.T) {
	previousVersion := buildinfo.Version
	previousBuildTime := buildinfo.BuildTime
	previousEnv := config.Config.App.Env
	previousAddr := config.Config.App.Addr
	t.Cleanup(func() {
		buildinfo.Version = previousVersion
		buildinfo.BuildTime = previousBuildTime
		config.Config.App.Env = previousEnv
		config.Config.App.Addr = previousAddr
	})

	buildinfo.Version = "v3.2.1"
	buildinfo.BuildTime = "2026-07-13T08:00:00Z"
	config.Config.App.Env = "production"
	config.Config.App.Addr = ":3000"

	banner := formatStartupBanner(startupState{
		mode: "API",
		relationalDB: migrator.Report{
			Backend: "PostgreSQL",
			Enabled: true,
			Version: 202607150003,
			Applied: true,
		},
		clickHouseDB:   migrator.Report{Backend: "ClickHouse"},
		listensForHTTP: true,
	})

	for _, want := range []string{
		"OpenFlare v3.2.1",
		"Environment: production",
		"Build time:  2026-07-13T08:00:00Z",
		"Database:    PostgreSQL (version 202607150003, upgraded)",
		"Analytics:   disabled",
		"Listening:   http://:3000",
		"Mode:        API",
	} {
		if !strings.Contains(banner, want) {
			t.Errorf("banner missing %q:\n%s", want, banner)
		}
	}
}
