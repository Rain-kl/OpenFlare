// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"errors"
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/routeidentity"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	whitelistRegexMatch  = "regex"
	whitelistPathLimit   = 1024
	whitelistRegexBudget = 8192
)

var whitelistRegexEscapes = strings.NewReplacer(`\\`, `\\`, `\b`, `(?-u:\b)`, `\B`, `(?-u:\B)`)

type whitelistEntryInput struct {
	// Exact hostname or a single left-most wildcard label.
	Domain string `json:"domain" binding:"required" maxLength:"253"`
	// One of prefix, suffix, regex, or exact.
	PathMatch string `json:"path_match" binding:"required" enums:"prefix,suffix,regex,exact"`
	// Case-sensitive URL path or regular expression. The query string is excluded.
	Path string `json:"path" binding:"required" maxLength:"1024"`
}

func saveWhitelistEntry(ctx context.Context, id uint, input whitelistEntryInput) (*model.OpenFlareWAFWhitelistEntry, error) {
	domain, err := routeidentity.NormalizeDomain(strings.TrimSuffix(strings.TrimSpace(input.Domain), "."))
	if err != nil {
		return nil, &RuleValidationError{Err: errors.New(errWhitelistDomain)}
	}
	path := strings.TrimSpace(input.Path)
	if err := validateWhitelistPath(input.PathMatch, path); err != nil {
		return nil, &RuleValidationError{Err: err}
	}
	entry := &model.OpenFlareWAFWhitelistEntry{ID: id, Domain: domain, PathMatch: input.PathMatch, Path: path}
	if err := repository.SaveOpenFlareWAFWhitelistEntry(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func validateWhitelistPath(mode, path string) error {
	if path == "" || len(path) > whitelistPathLimit || strings.ContainsFunc(path, unicode.IsControl) {
		return errors.New(errWhitelistPath)
	}
	switch mode {
	case whitelistRegexMatch:
		_, err := whitelistRegex(path)
		return err
	case "prefix", "exact":
		if !strings.HasPrefix(path, "/") {
			return errors.New(errWhitelistAbsolutePath)
		}
	case "suffix":
	default:
		return errors.New(errWhitelistPathMatch)
	}
	if strings.ContainsAny(path, "?#") || strings.ContainsFunc(path, unicode.IsSpace) {
		return errors.New(errWhitelistPath)
	}
	return nil
}

func whitelistRegex(path string) (string, error) {
	expr, err := syntax.Parse(path, syntax.Perl)
	if err != nil {
		return "", errors.New(errWhitelistRegex)
	}
	program, err := syntax.Compile(expr.Simplify())
	if err != nil {
		return "", errors.New(errWhitelistRegex)
	}
	// Bound the expanded program before distributing it to the Edge compiler.
	budget := whitelistRegexBudget
	for _, instruction := range program.Inst {
		budget -= 1 + len(instruction.Rune)
		if budget < 0 {
			return "", errors.New(errWhitelistRegexComplex)
		}
		for _, r := range instruction.Rune {
			if !utf8.ValidRune(r) {
				return "", errors.New(errWhitelistRegex)
			}
		}
	}
	// Canonical syntax expands character classes. The Edge needs no capture names
	// and must use Go's ASCII word boundary semantics.
	clearCaptureNames(expr)
	return whitelistRegexEscapes.Replace(expr.String()), nil
}

func clearCaptureNames(expr *syntax.Regexp) {
	expr.Name = ""
	for _, part := range expr.Sub {
		clearCaptureNames(part)
	}
}

// SnapshotWAFWhitelist loads the global whitelist with Edge-compatible path expressions.
func SnapshotWAFWhitelist(ctx context.Context) ([]edgeconfig.WAFWhitelistEntry, error) {
	entries, err := repository.ListOpenFlareWAFWhitelist(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := make([]edgeconfig.WAFWhitelistEntry, 0, len(entries))
	for _, entry := range entries {
		path := entry.Path
		if entry.PathMatch == whitelistRegexMatch {
			path, err = whitelistRegex(path)
			if err != nil {
				return nil, fmt.Errorf("compile WAF whitelist entry %d: %w", entry.ID, err)
			}
		}
		snapshot = append(snapshot, edgeconfig.WAFWhitelistEntry{
			Domain: entry.Domain, PathMatch: entry.PathMatch, Path: path,
		})
	}
	return snapshot, nil
}
