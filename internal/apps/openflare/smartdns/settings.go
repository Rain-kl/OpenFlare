// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	defaultSmartDNSUpdateRepo = "Rain-kl/OpenFlare"
	defaultSmartDNSTTL        = 60
	minPolicyTTL              = 10
	maxPolicyTTL              = 86400
	maxFQDNLength             = 253
	delegationLookupTimeout   = 5 * time.Second
)

var fqdnPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))+$`)

// SettingsView exposes SmartDNS control-plane settings to administrators.
type SettingsView struct {
	Enabled                 bool     `json:"enabled"`
	DelegationDomain        string   `json:"delegation_domain"`
	NSHosts                 []string `json:"ns_hosts"`
	DefaultTTL              int      `json:"default_ttl"`
	SnapshotIntervalSeconds int      `json:"snapshot_interval_seconds"`
	UpdateRepo              string   `json:"update_repo"`
	DiscoveryToken          string   `json:"discovery_token"`
}

// SettingsInput updates SmartDNS control-plane settings.
type SettingsInput struct {
	Enabled                 *bool    `json:"enabled"`
	DelegationDomain        *string  `json:"delegation_domain"`
	NSHosts                 []string `json:"ns_hosts"`
	DefaultTTL              *int     `json:"default_ttl"`
	SnapshotIntervalSeconds *int     `json:"snapshot_interval_seconds"`
	UpdateRepo              *string  `json:"update_repo"`
}

// DelegationVerifyResult reports live DNS NS/A resolution verification for the delegation domain.
type DelegationVerifyResult struct {
	DelegationDomain  string    `json:"delegation_domain"`
	ConfiguredNSHost  []string  `json:"configured_ns_hosts"`
	ObservedNSRecords []string  `json:"observed_ns_records"`
	MatchedNSHosts    []string  `json:"matched_ns_hosts"`
	MissingNSHosts    []string  `json:"missing_ns_hosts"`
	Verified          bool      `json:"verified"`
	CheckedAt         time.Time `json:"checked_at"`
	Error             string    `json:"error,omitempty"`
}

// DNSLookupFunc abstracts NS record lookups for delegation verification tests.
type DNSLookupFunc func(ctx context.Context, domain string) ([]string, error)

var lookupNSRecords DNSLookupFunc = func(ctx context.Context, domain string) ([]string, error) {
	recs, err := net.DefaultResolver.LookupNS(ctx, domain)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		if r != nil && strings.TrimSpace(r.Host) != "" {
			out = append(out, strings.TrimSuffix(strings.ToLower(strings.TrimSpace(r.Host)), "."))
		}
	}
	return out, nil
}

func readSmartDNSSettingsMap(ctx context.Context) SettingsView {
	view := SettingsView{
		Enabled:                 true,
		NSHosts:                 []string{},
		DefaultTTL:              defaultSmartDNSTTL,
		SnapshotIntervalSeconds: defaultSnapshotIntervalSec,
		UpdateRepo:              defaultSmartDNSUpdateRepo,
	}
	if db.DB(ctx) == nil {
		return view
	}
	readStr := func(key string) string {
		cfg, err := repository.GetSystemConfigByKey(ctx, key)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(cfg.Value)
	}
	if raw := readStr(model.ConfigKeySmartDNSEnabled); raw != "" {
		if b, err := strconv.ParseBool(raw); err == nil {
			view.Enabled = b
		}
	}
	view.DelegationDomain = normalizeDomain(readStr(model.ConfigKeySmartDNSDelegationDomain))
	if rawNS := readStr(model.ConfigKeySmartDNSNSHosts); rawNS != "" {
		var hosts []string
		if json.Unmarshal([]byte(rawNS), &hosts) == nil {
			view.NSHosts = normalizeDomainList(hosts)
		}
	}
	if rawTTL := readStr(model.ConfigKeySmartDNSDefaultTTL); rawTTL != "" {
		if ttl, err := strconv.Atoi(rawTTL); err == nil && ttl >= minPolicyTTL && ttl <= maxPolicyTTL {
			view.DefaultTTL = ttl
		}
	}
	if rawInt := readStr(model.ConfigKeySmartDNSSnapshotIntervalSeconds); rawInt != "" {
		if sec, err := strconv.Atoi(rawInt); err == nil && sec >= minSnapshotIntervalSec && sec <= maxSnapshotIntervalSec {
			view.SnapshotIntervalSeconds = sec
		}
	}
	if repo := readStr(model.ConfigKeySmartDNSUpdateRepo); repo != "" {
		view.UpdateRepo = repo
	}
	view.DiscoveryToken = readStr(model.ConfigKeySmartDNSDiscoveryToken)
	return view
}

// GetSettings returns the current SmartDNS configuration view, lazily initializing the discovery token if needed.
func GetSettings(ctx context.Context) (*SettingsView, error) {
	view := readSmartDNSSettingsMap(ctx)
	if strings.TrimSpace(view.DiscoveryToken) == "" && db.DB(ctx) != nil {
		tok, err := ensureSmartDNSDiscoveryToken(ctx)
		if err == nil {
			view.DiscoveryToken = tok
		}
	}
	return &view, nil
}

// SaveSettings validates and persists SmartDNS configuration changes, triggering snapshot invalidation.
func SaveSettings(ctx context.Context, input SettingsInput) (*SettingsView, error) {
	cur := readSmartDNSSettingsMap(ctx)
	if err := applySettingsInput(&cur, input); err != nil {
		return nil, err
	}
	updates, err := buildSettingsUpdates(cur)
	if err != nil {
		return nil, err
	}
	for k, v := range updates {
		if err := repository.SaveOrUpdateSystemConfig(ctx, k, v); err != nil {
			return nil, err
		}
	}
	NotifySnapshotDirty(ctx)
	return GetSettings(ctx)
}

func applySettingsInput(cur *SettingsView, input SettingsInput) error {
	if input.Enabled != nil {
		cur.Enabled = *input.Enabled
	}
	if input.DelegationDomain != nil {
		d := normalizeDomain(*input.DelegationDomain)
		if d != "" && (!fqdnPattern.MatchString(d) || len(d) > maxFQDNLength) {
			return newLogicError(http.StatusBadRequest, errDelegationDomainInvalid)
		}
		cur.DelegationDomain = d
	}
	if input.NSHosts != nil {
		hosts := normalizeDomainList(input.NSHosts)
		for _, h := range hosts {
			if !fqdnPattern.MatchString(h) || len(h) > maxFQDNLength {
				return newLogicError(http.StatusBadRequest, errSettingsInvalid)
			}
		}
		cur.NSHosts = hosts
	}
	if input.DefaultTTL != nil {
		if *input.DefaultTTL < minPolicyTTL || *input.DefaultTTL > maxPolicyTTL {
			return newLogicError(http.StatusBadRequest, errSettingsInvalid)
		}
		cur.DefaultTTL = *input.DefaultTTL
	}
	if input.SnapshotIntervalSeconds != nil {
		if *input.SnapshotIntervalSeconds < minSnapshotIntervalSec || *input.SnapshotIntervalSeconds > maxSnapshotIntervalSec {
			return newLogicError(http.StatusBadRequest, errSettingsInvalid)
		}
		cur.SnapshotIntervalSeconds = *input.SnapshotIntervalSeconds
	}
	if input.UpdateRepo != nil {
		repo := strings.TrimSpace(*input.UpdateRepo)
		if repo == "" {
			repo = defaultSmartDNSUpdateRepo
		}
		cur.UpdateRepo = repo
	}
	return nil
}

func buildSettingsUpdates(cur SettingsView) (map[string]string, error) {
	nsJSON, err := json.Marshal(cur.NSHosts)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		model.ConfigKeySmartDNSEnabled:                 strconv.FormatBool(cur.Enabled),
		model.ConfigKeySmartDNSDelegationDomain:        cur.DelegationDomain,
		model.ConfigKeySmartDNSNSHosts:                 string(nsJSON),
		model.ConfigKeySmartDNSDefaultTTL:              strconv.Itoa(cur.DefaultTTL),
		model.ConfigKeySmartDNSSnapshotIntervalSeconds: strconv.Itoa(cur.SnapshotIntervalSeconds),
		model.ConfigKeySmartDNSUpdateRepo:              cur.UpdateRepo,
	}, nil
}

// VerifyDelegation verifies public DNS NS delegation for the configured SmartDNS delegation domain.
func VerifyDelegation(ctx context.Context, overrideDomain string) (*DelegationVerifyResult, error) {
	settings := readSmartDNSSettingsMap(ctx)
	domain := normalizeDomain(overrideDomain)
	if domain == "" {
		domain = settings.DelegationDomain
	}
	if domain == "" || !fqdnPattern.MatchString(domain) {
		return nil, newLogicError(http.StatusBadRequest, errDelegationDomainInvalid)
	}

	lookupCtx, cancel := context.WithTimeout(ctx, delegationLookupTimeout)
	defer cancel()
	observed, err := lookupNSRecords(lookupCtx, domain)
	res := &DelegationVerifyResult{
		DelegationDomain:  domain,
		ConfiguredNSHost:  settings.NSHosts,
		ObservedNSRecords: observed,
		MatchedNSHosts:    []string{},
		MissingNSHosts:    []string{},
		CheckedAt:         time.Now().UTC(),
	}
	if err != nil {
		res.Error = err.Error()
		res.MissingNSHosts = append(res.MissingNSHosts, settings.NSHosts...)
		return res, nil
	}
	obsSet := make(map[string]bool, len(observed))
	for _, host := range observed {
		obsSet[host] = true
	}
	if len(settings.NSHosts) == 0 {
		res.Verified = len(observed) > 0
		res.MatchedNSHosts = observed
		return res, nil
	}
	for _, host := range settings.NSHosts {
		if obsSet[host] {
			res.MatchedNSHosts = append(res.MatchedNSHosts, host)
		} else {
			res.MissingNSHosts = append(res.MissingNSHosts, host)
		}
	}
	res.Verified = len(res.MatchedNSHosts) > 0 && len(res.MissingNSHosts) == 0
	return res, nil
}

func ensureSmartDNSDiscoveryToken(ctx context.Context) (string, error) {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySmartDNSDiscoveryToken)
	if err == nil && strings.TrimSpace(cfg.Value) != "" {
		return strings.TrimSpace(cfg.Value), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	tok, err := generateHexToken(discoveryTokenBytes)
	if err != nil {
		return "", err
	}
	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeySmartDNSDiscoveryToken, tok); err != nil {
		return "", err
	}
	return tok, nil
}

func generateHexToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func normalizeDomain(raw string) string {
	return strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "."), ".")
}

func normalizeDomainList(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		d := normalizeDomain(item)
		if d != "" && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}
