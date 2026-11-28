// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsprovider

import (
	"context"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/challenge/dns01"
)

type recordChallenge struct {
	adapter Adapter
	records sync.Map
}

type challengeRecord struct{ zone, id string }

func (c *recordChallenge) Present(domain, _, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)
	zoneName, err := dns01.FindZoneByFqdn(info.EffectiveFQDN)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	zone, err := c.adapter.FindZone(ctx, dnsName(zoneName))
	if err != nil {
		return err
	}
	record, err := c.adapter.CreateRecord(ctx, zone.ID, RecordInput{Type: "TXT", Name: dnsName(info.EffectiveFQDN), Content: info.Value, TTL: 600})
	if err != nil {
		return err
	}
	c.records.Store(keyAuth, challengeRecord{zone.ID, record.ID})
	return nil
}

func (c *recordChallenge) CleanUp(_, _, keyAuth string) error {
	stored, ok := c.records.Load(keyAuth)
	if !ok {
		return nil
	}
	record := stored.(challengeRecord)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := c.adapter.DeleteRecord(ctx, record.zone, record.id); err != nil {
		return err
	}
	c.records.Delete(keyAuth)
	return nil
}
