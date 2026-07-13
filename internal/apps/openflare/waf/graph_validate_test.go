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
		}, "恰好包含一个开始节点"},
		{"duplicate allow", func(g *RuleGraph) {
			g.Nodes = append(g.Nodes, RuleNode{ID: "allow-2", Type: RuleNodeAllow, Config: rawConfig(`{}`)})
		}, "恰好包含一个通过节点"},
		{"cycle", func(g *RuleGraph) {
			g.Edges[2].Target = "start"
		}, "规则图不能包含循环"},
		{"unreachable", func(g *RuleGraph) {
			g.Nodes = append(g.Nodes, RuleNode{ID: "orphan", Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403,"response_body":""}`)})
		}, "节点 orphan 无法从开始节点到达"},
		{"missing false edge", func(g *RuleGraph) { g.Edges = g.Edges[:2] }, "节点 match-1 的 false 出口未连接"},
		{"wrong handle", func(g *RuleGraph) { g.Edges[0].SourceHandle = "true" }, "边 e1 的源端口 true 不适用于节点 start"},
		{"same handle multiple targets", func(g *RuleGraph) {
			g.Edges = append(g.Edges, RuleEdge{ID: "e4", Source: "match-1", SourceHandle: "true", Target: "allow"})
		}, "节点 match-1 的 true 出口连接了多个目标"},
		{"unknown type", func(g *RuleGraph) { g.Nodes[1].Type = RuleNodeType("script") }, "节点 match-1 的类型 script 未知"},
		{"dangling target", func(g *RuleGraph) { g.Edges[1].Target = "missing" }, "边 e2 的目标节点 missing 不存在"},
		{"invalid ip", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"ips":["bad"]}`) }, "节点 match-1 的 IP bad 无效"},
		{"invalid cidr", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"cidrs":["bad"]}`) }, "节点 match-1 的 CIDR bad 无效"},
		{"missing ip group", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"ip_group_ids":[99]}`) }, "节点 match-1 引用的 IP 组 99 不存在"},
		{"invalid country", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeGeoMatch
			g.Nodes[1].Config = rawConfig(`{"countries":["USA"]}`)
		}, "节点 match-1 的国家代码 USA 无效"},
		{"invalid region", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodeGeoMatch
			g.Nodes[1].Config = rawConfig(`{"regions":["US-"]}`)
		}, "节点 match-1 的地区代码 US- 无效"},
		{"pow range", func(g *RuleGraph) {
			g.Nodes[1].Type = RuleNodePoW
			g.Nodes[1].Config = rawConfig(`{"algorithm":"fast","difficulty":17,"session_ttl":60,"challenge_ttl":30}`)
			g.Edges[1].SourceHandle = "next"
			g.Edges = g.Edges[:2]
		}, "节点 match-1 的 PoW 难度必须在 1-16 之间"},
		{"unknown config field", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`{"ips":[],"surprise":true}`) }, "节点 match-1 的配置无效"},
		{"null config", func(g *RuleGraph) { g.Nodes[1].Config = rawConfig(`null`) }, "节点 match-1 的配置无效"},
		{"too many nodes", func(g *RuleGraph) {
			for i := 0; i < 125; i++ {
				g.Nodes = append(g.Nodes, RuleNode{ID: strings.Repeat("x", i+1), Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403}`)})
			}
		}, "规则图节点数不能超过 128"},
		{"too many edges", func(g *RuleGraph) {
			for i := 0; i < 254; i++ {
				g.Edges = append(g.Edges, RuleEdge{ID: strings.Repeat("e", i+5), Source: "start", SourceHandle: "next", Target: "allow"})
			}
		}, "规则图边数不能超过 256"},
		{"graph too large", func(g *RuleGraph) {
			g.Nodes[0].Label = strings.Repeat("x", 256*1024)
		}, "规则图大小不能超过 256 KiB"},
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
	if err == nil || !strings.Contains(err.Error(), "节点 start 不存在通往终止节点的路径") {
		t.Fatalf("validateTerminalPaths() error = %v", err)
	}
}
