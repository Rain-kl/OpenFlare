// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsprovider

import (
	"context"
	"fmt"
)

// DelegationChanges reports which records a delegation reconciliation modified.
type DelegationChanges struct {
	Created int `json:"created"`
	Removed int `json:"removed"`
}

// ReconcileDelegation makes an NS record set match the configured nameservers.
// New nameservers are installed before stale records are removed.
func ReconcileDelegation(ctx context.Context, client Client, zone Zone, domain string, hosts []string, ttl int) (DelegationChanges, error) {
	changes := DelegationChanges{}
	records, err := client.ListRecords(ctx, zone.ID, domain, "NS")
	if err != nil {
		return changes, err
	}
	desired := make(map[string]bool, len(hosts))
	for _, host := range hosts {
		desired[dnsName(host)] = false
	}
	var stale []DNSRecord
	for _, record := range records {
		host := dnsName(record.Content)
		if record.Type != "NS" || dnsName(record.Name) != dnsName(domain) {
			continue
		}
		found, ok := desired[host]
		if ok && !found {
			desired[host] = true
			continue
		}
		stale = append(stale, record)
	}
	for _, host := range hosts {
		if desired[dnsName(host)] {
			continue
		}
		_, err = client.CreateRecord(ctx, zone.ID, RecordInput{Type: "NS", Name: domain, Content: dnsName(host), TTL: ttl})
		if err != nil {
			return changes, fmt.Errorf("create NS record: %w", err)
		}
		changes.Created++
		desired[dnsName(host)] = true
	}
	for _, record := range stale {
		if err = client.DeleteRecord(ctx, zone.ID, record.ID); err != nil {
			return changes, fmt.Errorf("remove obsolete NS record: %w", err)
		}
		changes.Removed++
	}
	return changes, nil
}
