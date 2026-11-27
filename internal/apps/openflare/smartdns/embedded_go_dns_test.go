// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedGoDNSServerAQuadANSAndECSScope(t *testing.T) {
	cleanup := setupSmartDNSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-fast", Name: "HK Fast", IP: "203.0.113.10", Status: "online", NodeType: "edge_node", LastSeenAt: &now, CapacityWeight: 100,
	}))
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-v6", Name: "HK IPv6", IP: "2001:db8::10", Status: "online", NodeType: "edge_node", LastSeenAt: &now, CapacityWeight: 100,
	}))

	del := "edge.example.com"
	_, err := SaveSettings(ctx, SettingsInput{
		DelegationDomain: &del,
		NSHosts:          []string{"ns1.edge.example.com"},
	})
	require.NoError(t, err)

	_, err = CreatePolicy(ctx, PolicyInput{
		Name:         "Default Policy",
		StrategyType: model.DNSPolicyStrategySmart,
		ScopeType:    model.DNSPolicyScopeAll,
		TTL:          60,
		MaxAnswers:   1,
		IsDefault:    boolPtr(true),
		Config:       defaultPolicyConfig(model.DNSPolicyStrategySmart),
	})
	require.NoError(t, err)

	_, err = DefaultSnapshotManager().Build(ctx)
	require.NoError(t, err)

	srv := NewEmbeddedGoDNSServer("127.0.0.1:0", defaultSnapshotManager)
	require.NoError(t, srv.Start(ctx))
	defer func() { _ = srv.Stop(context.Background()) }()

	client := &dns.Client{Net: "udp", Timeout: 3 * time.Second}

	// 1. Query A with IPv4 ECS (/24 scope expected in reply).
	msgA := new(dns.Msg)
	msgA.SetQuestion("cdn.edge.example.com.", dns.TypeA)
	msgA.SetEdns0(4096, false)
	opt := msgA.IsEdns0()
	opt.Option = append(opt.Option, &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Family:        1,
		SourceNetmask: 24,
		Address:       net.ParseIP("113.108.10.5"),
	})

	respA, _, err := client.Exchange(msgA, srv.Addr())
	require.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, respA.Rcode)
	assert.True(t, respA.Authoritative)
	require.Len(t, respA.Answer, 1)
	aRec, ok := respA.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "203.0.113.10", aRec.A.String())

	var replyScope uint8
	if respOPT := respA.IsEdns0(); respOPT != nil {
		for _, o := range respOPT.Option {
			if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
				replyScope = ecs.SourceScope
			}
		}
	}
	assert.Equal(t, uint8(24), replyScope)

	// 2. Query AAAA with IPv6 ECS (/48 scope expected).
	msgAAAA := new(dns.Msg)
	msgAAAA.SetQuestion("cdn.edge.example.com.", dns.TypeAAAA)
	msgAAAA.SetEdns0(4096, false)
	msgAAAA.IsEdns0().Option = append(msgAAAA.IsEdns0().Option, &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Family:        2,
		SourceNetmask: 56,
		Address:       net.ParseIP("2408:8207::1"),
	})
	respAAAA, _, err := client.Exchange(msgAAAA, srv.Addr())
	require.NoError(t, err)
	require.Len(t, respAAAA.Answer, 1)
	aaaaRec, ok := respAAAA.Answer[0].(*dns.AAAA)
	require.True(t, ok)
	assert.Equal(t, "2001:db8::10", aaaaRec.AAAA.String())

	var v6Scope uint8
	if respOPT := respAAAA.IsEdns0(); respOPT != nil {
		for _, o := range respOPT.Option {
			if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
				v6Scope = ecs.SourceScope
			}
		}
	}
	assert.Equal(t, uint8(48), v6Scope)

	// 3. Query NS & SOA on delegation apex.
	msgNS := new(dns.Msg)
	msgNS.SetQuestion("edge.example.com.", dns.TypeNS)
	respNS, _, err := client.Exchange(msgNS, srv.Addr())
	require.NoError(t, err)
	require.Len(t, respNS.Answer, 1)

	// 4. Out-of-zone query returns REFUSED.
	msgRefused := new(dns.Msg)
	msgRefused.SetQuestion("unrelated.invalid.", dns.TypeA)
	respRefused, _, err := client.Exchange(msgRefused, srv.Addr())
	require.NoError(t, err)
	assert.Equal(t, dns.RcodeRefused, respRefused.Rcode)

	// 5. Repeat query hits L1 cache and preserves metrics counters.
	respA2, _, err := client.Exchange(msgA, srv.Addr())
	require.NoError(t, err)
	require.Len(t, respA2.Answer, 1)
	assert.GreaterOrEqual(t, srv.metrics.cacheHits.Load(), uint64(1))
}
