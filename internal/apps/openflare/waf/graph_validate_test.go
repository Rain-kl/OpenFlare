// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func rawConfig(value string) json.RawMessage { return json.RawMessage(value) }

func TestDefaultRuleGraph(t *testing.T) {
	graph := DefaultRuleGraph()
	if err := ValidateRuleGraph(context.Background(), graph, nil); err != nil {
		t.Fatalf("default graph must be valid: %v", err)
	}
	if graph.SchemaVersion != 1 || len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("unexpected default graph: %#v", graph)
	}
}

func TestValidateRuleGraph(t *testing.T) {
	validBranch := func() RuleGraph {
		return RuleGraph{SchemaVersion: 1, Nodes: []RuleNode{
			{ID: "start", Type: RuleNodeStart, Config: rawConfig(`{}`)},
			{ID: "match-1", Type: RuleNodeIPMatch, Config: rawConfig(`{"ips":["192.0.2.1"],"cidrs":["2001:db8::/32"],"ip_group_ids":[7]}`)},
			{ID: "allow", Type: RuleNodeAllow, Config: rawConfig(`{}`)},
			{ID: "block", Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403,"response_body":"denied"}`)},
		}, Edges: []RuleEdge{
			{ID: "e1", Source: "start", SourceHandle: "next", Target: "match-1"},
			{ID: "e2", Source: "match-1", SourceHandle: "true", Target: "block"},
			{ID: "e3", Source: "match-1", SourceHandle: "false", Target: "allow"},
		}}
	}

	tests := []struct {
		name   string
		mutate func(*RuleGraph)
		want   string
	}{
		{"duplicate start", func(g *RuleGraph) {
			g.Nodes = append(g.Nodes, RuleNode{ID: "start-2", Type: RuleNodeStart, Config: rawConfig(`{}`)})
		}, "rule graph must contain exactly one start node"},
		{"duplicate allow", func(g *RuleGraph) {
			g.Nodes = append(g.Nodes, RuleNode{ID: "allow-2", Type: RuleNodeAllow, Config: rawConfig(`{}`)})
		}, "rule graph must contain exactly one allow node"},
		{"cycle", func(g *RuleGraph) {
			g.Edges[2].Target = "start"
		}, "rule graph must not contain cycles"},
		{"unreachable", func(g *RuleGraph) {
			g.Nodes = append(g.Nodes, RuleNode{ID: "orphan", Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403,"response_body":""}`)})
		}, "node orphan is not reachable from the start node"},
		{"missing false edge", func(g *RuleGraph) { g.Edges = g.Edges[:2] }, "node match-1 output false is not connected"},
		{"wrong handle", func(g *RuleGraph) { g.Edges[0].SourceHandle = "true" }, "edge e1 source port true is not valid for node start"},
		{"same handle multiple targets", func(g *RuleGraph) {
			g.Edges = append(g.Edges, RuleEdge{ID: "e4", Source: "match-1", SourceHandle: "true", Target: "allow"})
		}, "node match-1 output true is connected to multiple targets"},
		{"unknown type", func(g *RuleGraph) { g.Nodes[1].Type = RuleNodeType("script") }, "node match-1 has unknown type script"},
		{"dangling target", func(g *RuleGraph) { g.Edges[1].Target = "missing" }, "edge e2 references missing target node missing"},
		{"invalid ip", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"ips":["bad"]}`) }, "node match-1 contains invalid IP bad"},
		{"invalid cidr", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"cidrs":["bad"]}`) }, "node match-1 contains invalid CIDR bad"},
		{"missing ip group", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"ip_group_ids":[99]}`) }, "node match-1 references missing IP group 99"},
		{"invalid country", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeGeoMatch
			g.Nodes[1].Config = rawConfig(`{"countries":["USA"]}`)
		}, "node match-1 contains invalid country code USA"},
		{"invalid region", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeGeoMatch
			g.Nodes[1].Config = rawConfig(`{"regions":["US-"]}`)
		}, "node match-1 contains invalid region code US-"},
		{"pow range", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodePoW
			g.Nodes[1].Config = rawConfig(`{"algorithm":"fast","difficulty":17,"session_ttl":60,"challenge_ttl":30}`)
			g.Edges[1].SourceHandle = "next"
			g.Edges = g.Edges[:2]
		}, "PoW difficulty for node match-1 must be between 1 and 16"},
		{"invalid ua browser", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeUACheck
			g.Nodes[1].Config = rawConfig(`{"browsers":["NotABrowser"],"match_mode":"or"}`)
		}, "node match-1 contains invalid browser tag NotABrowser"},
		{"invalid ua match mode", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeUACheck
			g.Nodes[1].Config = rawConfig(`{"match_mode":"xor"}`)
		}, "match mode for node match-1 must be and or or"},
		{"invalid ua custom regex", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeUACheck
			g.Nodes[1].Config = rawConfig(`{"block_custom_ua":true,"custom_ua_patterns":["("]}`)
		}, "invalid custom User-Agent pattern in node match-1"},
		{"custom ua requires patterns", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeUACheck
			g.Nodes[1].Config = rawConfig(`{"block_custom_ua":true}`)
		}, "node match-1 requires at least one pattern when custom User-Agent blocking is enabled"},
		{"unknown config field", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"ips":[],"surprise":true}`) }, "invalid configuration for node match-1"},
		{"null config", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`null`) }, "invalid configuration for node match-1"},
		{"too many nodes", func(g *RuleGraph) {
			for i := 0; i < 125; i++ {
				g.Nodes = append(g.Nodes, RuleNode{ID: strings.Repeat("x", i+1), Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403}`)})
			}
		}, "rule graph must not contain more than 128 nodes"},
		{"too many edges", func(g *RuleGraph) {
			for i := 0; i < 254; i++ {
				g.Edges = append(g.Edges, RuleEdge{ID: strings.Repeat("e", i+5), Source: "start", SourceHandle: "next", Target: "allow"})
			}
		}, "rule graph must not contain more than 256 edges"},
		{"graph too large", func(g *RuleGraph) {
			g.Nodes[0].Label = strings.Repeat("x", 256*1024)
		}, "rule graph must not exceed 256 KiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := validBranch()
			tt.mutate(&graph)
			exists := func(_ context.Context, id uint) (bool, error) { return id == 7, nil }
			err := ValidateRuleGraph(context.Background(), graph, exists)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateRuleGraph() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestValidateRuleGraphDetectsNodeWithoutTerminalPath(t *testing.T) {
	nodes := []RuleNode{{ID: "start", Type: RuleNodeStart}, {ID: "match", Type: RuleNodeIPMatch}, {ID: "allow", Type: RuleNodeAllow}}
	outgoing := map[string][]RuleEdge{"start": {{Source: "start", Target: "match"}}}
	err := validateTerminalPaths(nodes, outgoing)
	if err == nil || !strings.Contains(err.Error(), "node start has no path to a terminal node") {
		t.Fatalf("validateTerminalPaths() error = %v", err)
	}
}
