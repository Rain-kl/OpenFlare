// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

const (
	errAttackGroupName       = "attack protection group name must contain 1–100 characters"
	errAttackGroupLevels     = "attack protection category or level is invalid"
	errAttackGroupReference  = "attack protection group %d was not found"
	errAttackRuleOverride    = "attack protection rule override %q is invalid"
	errAttackLogFilters      = "attack log filters are invalid"
	errAttackLogRead         = "Unable to read attack logs"
	errAttackLogNotFound     = "Attack log not found"
	errWhitelistDomain       = "enter a valid hostname or wildcard domain"
	errWhitelistPath         = "enter a valid path or regular expression up to 1024 bytes"
	errWhitelistAbsolutePath = "exact and prefix paths must start with /"
	errWhitelistPathMatch    = "invalid path match type"
	errWhitelistRegex        = "invalid path regular expression"
	errWhitelistRegexComplex = "path regular expression is too complex"
)
