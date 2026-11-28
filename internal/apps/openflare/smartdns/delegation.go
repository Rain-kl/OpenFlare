// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/dnsaccount"
	"github.com/Rain-kl/Wavelet/internal/openflare/dnsprovider"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	delegationWriteTimeout       = 45 * time.Second
	delegationRecordTTL          = 600
	errDelegationNSRequired      = "save a delegation domain and at least one nameserver first"
	errDelegationScope           = "no unique DNS account covers the parent zone of the delegation domain"
	errDelegationUnsupported     = "one-click delegation supports Cloudflare and Alibaba Cloud DNS"
	errDelegationSettingsChanged = "delegation settings changed; reload them before applying"
	errDelegationWriteFailed     = "could not apply SmartDNS NS records; check the DNS account and retry"
)

var delegationMu sync.Mutex

// DelegationApplyInput identifies the saved settings confirmed by the operator.
type DelegationApplyInput struct {
	Domain string   `json:"delegation_domain"`
	Hosts  []string `json:"ns_hosts"`
}

// DelegationApplyResult describes the parent zone and applied NS changes.
type DelegationApplyResult struct {
	Provider string `json:"provider"`
	Zone     string `json:"zone"`
	dnsprovider.DelegationChanges
}

// ApplyDelegation reconciles the saved NS records at the provider of the parent zone.
func ApplyDelegation(ctx context.Context, input DelegationApplyInput) (*DelegationApplyResult, error) {
	delegationMu.Lock()
	defer delegationMu.Unlock()
	settings := readSmartDNSSettingsMap(ctx)
	if settings.DelegationDomain == "" || len(settings.NSHosts) == 0 {
		return nil, newLogicError(http.StatusBadRequest, errDelegationNSRequired)
	}
	if input.Domain != settings.DelegationDomain || !slices.Equal(input.Hosts, settings.NSHosts) {
		return nil, newLogicError(http.StatusConflict, errDelegationSettingsChanged)
	}
	accounts, err := repository.ListDNSAccounts(ctx)
	if err != nil {
		return nil, err
	}
	route, err := dnsaccount.Resolve(accounts, settings.DelegationDomain, true)
	if err != nil || route == nil {
		return nil, newLogicError(http.StatusBadRequest, errDelegationScope)
	}
	if !dnsprovider.SupportsDelegation(route.Account.Type) {
		return nil, newLogicError(http.StatusBadRequest, errDelegationUnsupported)
	}
	client, err := route.Open()
	if err != nil {
		logger.WarnF(ctx, "SmartDNS delegation account unavailable: account_id=%d error=%v", route.Account.ID, err)
		return nil, newLogicError(http.StatusBadRequest, errDelegationWriteFailed)
	}
	writeCtx, cancel := context.WithTimeout(ctx, delegationWriteTimeout)
	defer cancel()
	changes, err := dnsprovider.ReconcileDelegation(writeCtx, client, route.Zone, settings.DelegationDomain, settings.NSHosts, delegationRecordTTL)
	if err != nil {
		logger.WarnF(ctx, "SmartDNS delegation failed: account_id=%d domain=%s created=%d removed=%d error=%v", route.Account.ID, settings.DelegationDomain, changes.Created, changes.Removed, err)
		return nil, newLogicError(http.StatusInternalServerError, errDelegationWriteFailed)
	}
	return &DelegationApplyResult{Provider: route.Account.Type, Zone: route.Zone.Name, DelegationChanges: changes}, nil
}
