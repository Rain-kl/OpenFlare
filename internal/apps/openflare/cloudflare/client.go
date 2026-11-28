// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cloudflare reconciles OpenFlare DNS pointing groups.
package cloudflare

import "github.com/Rain-kl/Wavelet/internal/openflare/dnsprovider"

type (
	// Zone identifies a provider DNS zone.
	Zone = dnsprovider.Zone
	// DNSRecord is the provider record returned by reconciliation.
	DNSRecord = dnsprovider.DNSRecord
	// RecordInput is the desired record used by pointing groups.
	RecordInput = dnsprovider.RecordInput
	// Client is the record client used by pointing groups.
	Client = dnsprovider.Client
)

var (
	// NewHTTPClient creates the Cloudflare adapter.
	NewHTTPClient = dnsprovider.NewHTTPClient
	// WithBaseURL overrides the Cloudflare API endpoint.
	WithBaseURL = dnsprovider.WithBaseURL
	// WithHTTPClient overrides the Cloudflare HTTP client.
	WithHTTPClient = dnsprovider.WithHTTPClient
)
