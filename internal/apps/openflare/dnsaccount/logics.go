// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dnsaccount discovers and routes verified DNS account scopes.
package dnsaccount

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/dnsprovider"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	scopeProbeTimeout     = 20 * time.Second
	errAccountUnavailable = "DNS account API is unavailable"
	errAmbiguousScope     = "multiple DNS accounts have the same matching zone"
)

// OpenAdapter opens the encrypted credential and selects its provider adapter.
func OpenAdapter(account *model.DNSAccount) (dnsprovider.Adapter, error) {
	authorization, err := credential.Open(account.Authorization)
	if err != nil {
		return nil, err
	}
	return dnsprovider.New(account.Type, authorization)
}

// Refresh updates the API status and authorized zones for one DNS account.
func Refresh(ctx context.Context, id uint) (*model.DNSAccount, error) {
	account, err := repository.GetDNSAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = refresh(ctx, account); err != nil {
		return nil, err
	}
	return repository.GetDNSAccountByID(ctx, id)
}

func refresh(ctx context.Context, account *model.DNSAccount) error {
	started := time.Now().UTC()
	probe, cancel := context.WithTimeout(ctx, scopeProbeTimeout)
	defer cancel()
	adapter, err := OpenAdapter(account)
	scopes := make([]model.DNSAccountScope, 0)
	if err == nil {
		var listed []dnsprovider.Zone
		listed, err = adapter.ListZones(probe)
		scopes = append(scopes, listed...)
	}
	status := model.DNSAccountStatusReady
	if err != nil {
		status = model.DNSAccountStatusError
		logger.WarnF(ctx, "[DNS accounts] API verification failed: account_id=%d provider=%s error=%v", account.ID, account.Type, err)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Name < scopes[j].Name })
	if err = repository.SaveDNSAccountVerification(ctx, account, started, status, scopes); err != nil {
		return err
	}
	account.VerificationStatus = status
	return nil
}

// Route contains the credential and zone selected for a DNS operation.
type Route struct {
	Account *model.DNSAccount
	Zone    dnsprovider.Zone
}

// Resolve matches the most specific zone. Existing record ownership and the
// migrated Cloudflare connection resolve equal-specificity matches.
func Resolve(accounts []model.DNSAccount, name string, strictParent bool, preferred ...uint) (*Route, error) {
	var matches []Route
	longest := 0
	for i := range accounts {
		account := &accounts[i]
		zone := dnsprovider.ZoneForName(account.Scopes, name, strictParent)
		if zone == nil || len(zone.Name) < longest {
			continue
		}
		if len(zone.Name) > longest {
			matches = matches[:0]
			longest = len(zone.Name)
		}
		matches = append(matches, Route{Account: account, Zone: *zone})
	}
	if len(matches) == 0 {
		return nil, nil
	}
	for _, id := range preferred {
		for _, route := range matches {
			if route.Account.ID == id {
				return &route, nil
			}
		}
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%s: %s", errAmbiguousScope, matches[0].Zone.Name)
	}
	return &matches[0], nil
}

// Open verifies that the selected scope passed its latest API check.
func (r *Route) Open() (dnsprovider.Adapter, error) {
	if r.Account.VerificationStatus != model.DNSAccountStatusReady {
		return nil, errors.New(errAccountUnavailable)
	}
	return OpenAdapter(r.Account)
}
