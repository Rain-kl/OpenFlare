// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// OpenFlareWAFWhitelistEntry exempts requests matching both its domain and path.
type OpenFlareWAFWhitelistEntry struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	Domain    string `json:"domain"`
	PathMatch string `json:"path_match"`
	Path      string `json:"path"`
}

// TableName returns the global WAF whitelist table.
func (OpenFlareWAFWhitelistEntry) TableName() string { return "of_waf_request_whitelist" }
