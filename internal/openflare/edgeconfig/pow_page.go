// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package edgeconfig

import (
	_ "embed"
	"fmt"
)

// PoWPageSupportPath stores the published challenge page independently of Lua.
const PoWPageSupportPath = "pow/page.json"

// PoWPageConfig controls the challenge document and its reusable widget.
type PoWPageConfig struct {
	Template string `json:"template"`
	Widget   string `json:"widget"`
	HTML     string `json:"html,omitempty"`
}

//go:embed pow_templates/classic.html
var classicPoWPage string

//go:embed pow_templates/shield.html
var shieldPoWPage string

// DefaultPoWPageHTML returns one of the built-in challenge templates.
func DefaultPoWPageHTML(template string) string {
	if template == "classic" {
		return withBrandMark(classicPoWPage)
	}
	return withBrandMark(shieldPoWPage)
}

// ValidatePoWPageConfig validates the administrator's page configuration.
func ValidatePoWPageConfig(cfg PoWPageConfig) error {
	if cfg.Template != "" && cfg.Template != "classic" && cfg.Template != "shield" {
		return fmt.Errorf("PoW page template must be classic or shield")
	}
	if cfg.Widget != "" && cfg.Widget != "progress" && cfg.Widget != "turnstile" {
		return fmt.Errorf("PoW widget must be progress or turnstile")
	}
	if len(cfg.HTML) > 256<<10 {
		return fmt.Errorf("PoW page HTML must not exceed 256 KiB")
	}
	return nil
}
