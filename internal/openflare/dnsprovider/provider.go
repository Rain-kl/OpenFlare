// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dnsprovider defines DNS provider adapters and their capabilities.
package dnsprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"

	"github.com/go-acme/lego/v4/challenge"
	legocf "github.com/go-acme/lego/v4/providers/dns/cloudflare"
)

// Zone is a DNS zone.
type Zone = model.DNSAccountScope

// DNSRecord is a DNS record.
type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

// RecordInput is the desired DNS record.
type RecordInput struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

// Client describes the record operations used by pointing reconciliation.
type Client interface {
	VerifyToken(context.Context) error
	FindZone(context.Context, string) (*Zone, error)
	GetRecord(context.Context, string, string) (*DNSRecord, error)
	ListRecords(context.Context, string, string, string) ([]DNSRecord, error)
	CreateRecord(context.Context, string, RecordInput) (*DNSRecord, error)
	UpdateRecord(context.Context, string, string, RecordInput) (*DNSRecord, error)
	DeleteRecord(context.Context, string, string) error
}

// Adapter provides zone discovery and record operations for one credential.
type Adapter interface {
	Client
	ListZones(context.Context) ([]Zone, error)
}

// Provider identifiers stored in DNS accounts.
const (
	Cloudflare   = "cloudflare"
	AliDNS       = "alidns"
	TencentCloud = "tencentcloud"
)

type credentials struct {
	APIToken        string `json:"api_token"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SecretID        string `json:"secret_id"`
	SecretKey       string `json:"secret_key"`
}

type strategy struct {
	required   func(credentials) bool
	proxy      bool
	adapter    func(credentials) (Adapter, error)
	challenge  func(credentials) (challenge.Provider, error)
	delegation bool
}

// Registry maps persisted provider identifiers to their adapter and DNS challenge strategy.
var registry = map[string]strategy{
	Cloudflare: {
		required: func(c credentials) bool { return nonempty(c.APIToken) },
		proxy:    true,
		adapter:  func(c credentials) (Adapter, error) { return NewHTTPClient(c.APIToken), nil },
		challenge: func(c credentials) (challenge.Provider, error) {
			config := legocf.NewDefaultConfig()
			config.AuthToken = c.APIToken
			return legocf.NewDNSProviderConfig(config)
		},
		delegation: true,
	},
	AliDNS: {
		required:   func(c credentials) bool { return nonempty(c.AccessKeyID, c.AccessKeySecret) },
		adapter:    newAliDNS,
		delegation: true,
	},
	TencentCloud: {
		required: func(c credentials) bool { return nonempty(c.SecretID, c.SecretKey) },
		adapter:  newTencentDNS,
	},
}

// SupportsDelegation reports whether the provider supports one-click NS delegation.
func SupportsDelegation(provider string) bool { return registry[provider].delegation }

// SupportsProxy reports whether records support CDN proxying.
func SupportsProxy(provider string) bool { return registry[provider].proxy }

func parseCredentials(provider, authorization string) (strategy, credentials, error) {
	selected, ok := registry[provider]
	var c credentials
	if !ok {
		return selected, c, fmt.Errorf("unsupported DNS provider: %s", provider)
	}
	if json.Unmarshal([]byte(authorization), &c) != nil {
		return selected, c, errors.New("invalid DNS provider credentials")
	}
	if !selected.required(c) {
		return selected, c, errors.New("DNS provider credentials are incomplete")
	}
	return selected, c, nil
}

// ValidateCredentials checks that the provider has the required credential fields.
func ValidateCredentials(provider, authorization string) error {
	_, _, err := parseCredentials(provider, authorization)
	return err
}

// New opens the adapter for a saved DNS provider credential.
func New(provider, authorization string) (Adapter, error) {
	selected, c, err := parseCredentials(provider, authorization)
	if err != nil {
		return nil, err
	}
	return selected.adapter(c)
}

// Challenge creates the DNS-01 strategy for a provider credential.
func Challenge(provider, authorization string) (challenge.Provider, error) {
	selected, c, err := parseCredentials(provider, authorization)
	if err != nil {
		return nil, err
	}
	if selected.challenge != nil {
		return selected.challenge(c)
	}
	adapter, err := selected.adapter(c)
	if err != nil {
		return nil, err
	}
	return &recordChallenge{adapter: adapter}, nil
}

// ZoneForName returns the most specific zone containing a record.
func ZoneForName(zones []Zone, name string, strictParent bool) *Zone {
	var best *Zone
	name = dnsName(name)
	for i := range zones {
		zone := dnsName(zones[i].Name)
		if zone != "" && ((!strictParent && name == zone) || strings.HasSuffix(name, "."+zone)) &&
			(best == nil || len(zone) > len(dnsName(best.Name))) {
			best = &zones[i]
		}
	}
	return best
}

func dnsName(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}

func relativeName(zone, name string) (string, error) {
	zone, name = dnsName(zone), dnsName(name)
	if zone == name {
		return "@", nil
	}
	if rr, ok := strings.CutSuffix(name, "."+zone); ok && rr != "" {
		return rr, nil
	}
	return "", errors.New("DNS record is outside the zone")
}

func fullName(zone, rr string) string {
	if rr == "@" || rr == "" {
		return dnsName(zone)
	}
	return dnsName(rr) + "." + dnsName(zone)
}

const defaultProviderTTL = 600

func recordTTL(ttl int) int {
	if ttl <= 1 {
		return defaultProviderTTL
	}
	return ttl
}

func nonempty(fields ...string) bool {
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return false
		}
	}
	return true
}

func value[T any](ptr *T) T {
	if ptr != nil {
		return *ptr
	}
	var zero T
	return zero
}

func findExactZone(ctx context.Context, adapter Adapter, name string) (*Zone, error) {
	zones, err := adapter.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	zone := ZoneForName(zones, name, false)
	if zone == nil || dnsName(zone.Name) != dnsName(name) {
		return nil, fmt.Errorf("DNS zone %s is unavailable", name)
	}
	return zone, nil
}

func writtenRecord(id string, in RecordInput) (*DNSRecord, error) {
	if id == "" {
		return nil, errors.New("DNS provider did not return a record identifier")
	}
	return &DNSRecord{ID: id, Type: in.Type, Name: in.Name, Content: in.Content, TTL: recordTTL(in.TTL)}, nil
}
