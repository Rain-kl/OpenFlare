// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCompileRuleGraph(t *testing.T) {
	graph := RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "allow", Type: RuleNodeAllow, Label: "通过", Position: RulePosition{X: 900, Y: 100}, Config: rawConfig(`{}`)},
		{ID: "match", Type: RuleNodeIPMatch, Label: "IP 匹配", Position: RulePosition{X: 320, Y: 100}, Config: rawConfig(`{"ips":["192.0.2.2","192.0.2.1"],"cidrs":["2001:db8:2::/48","2001:db8:1::/48"],"ip_group_ids":[7,2,7]}`)},
		{ID: "start", Type: RuleNodeStart, Label: "开始", Position: RulePosition{X: 0, Y: 100}, Config: rawConfig(`{}`)},
		{ID: "block", Type: RuleNodeBlock, Label: "阻止", Position: RulePosition{X: 900, Y: 300}, Config: rawConfig(`{"status_code":403,"response_body":"denied"}`)},
	}, Edges: []RuleEdge{
		{ID: "edge-false", Source: "match", SourceHandle: "false", Target: "block"},
		{ID: "edge-start", Source: "start", SourceHandle: "next", Target: "match"},
		{ID: "edge-true", Source: "match", SourceHandle: "true", Target: "allow"},
	}}

	compiled, err := CompileRuleGraph(graph)
	if err != nil {
		t.Fatalf("CompileRuleGraph() error = %v", err)
	}
	if compiled.Entry != "start" {
		t.Fatalf("entry = %q, want start", compiled.Entry)
	}
	if len(compiled.Nodes) != 4 || compiled.Nodes["match"].Next["true"] != "allow" || compiled.Nodes["match"].Next["false"] != "block" {
		t.Fatalf("unexpected compiled node index: %#v", compiled.Nodes)
	}

	raw, err := json.Marshal(compiled)
	if err != nil {
		t.Fatalf("marshal compiled graph: %v", err)
	}
	want := `{"entry":"start","nodes":{"allow":{"type":"allow","config":{}},"block":{"type":"block","config":{"status_code":403,"response_body":"denied"}},"match":{"type":"ip_match","config":{"ips":["192.0.2.1","192.0.2.2"],"cidrs":["2001:db8:1::/48","2001:db8:2::/48"],"ip_group_ids":[2,7]},"next":{"false":"block","true":"allow"}},"start":{"type":"start","config":{},"next":{"next":"match"}}}}`
	if string(raw) != want {
		t.Fatalf("compiled JSON = %s\nwant          = %s", raw, want)
	}
}

func TestCompileRuleGraphIsDeterministicForNodeAndEdgeOrder(t *testing.T) {
	first := RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "start", Type: RuleNodeStart, Config: rawConfig(`{}`)},
		{ID: "match", Type: RuleNodeIPMatch, Config: rawConfig(`{"ip_group_ids":[7,2]}`)},
		{ID: "allow", Type: RuleNodeAllow, Config: rawConfig(`{}`)},
		{ID: "block", Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403}`)},
	}, Edges: []RuleEdge{
		{ID: "start-match", Source: "start", SourceHandle: "next", Target: "match"},
		{ID: "match-allow", Source: "match", SourceHandle: "true", Target: "allow"},
		{ID: "match-block", Source: "match", SourceHandle: "false", Target: "block"},
	}}
	second := RuleGraph{SchemaVersion: first.SchemaVersion,
		Nodes: []RuleNode{first.Nodes[3], first.Nodes[2], first.Nodes[1], first.Nodes[0]},
		Edges: []RuleEdge{first.Edges[2], first.Edges[1], first.Edges[0]},
	}

	a, err := CompileRuleGraph(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CompileRuleGraph(second)
	if err != nil {
		t.Fatal(err)
	}
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	if string(aJSON) != string(bJSON) {
		t.Fatalf("equivalent graphs compiled differently:\n%s\n%s", aJSON, bJSON)
	}
}

func TestCompileRuleGraphDoesNotMutateInput(t *testing.T) {
	graph := RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "start", Type: RuleNodeStart, Config: rawConfig(`{}`)},
		{ID: "match", Type: RuleNodeIPMatch, Config: rawConfig(`{"ips":["192.0.2.2","192.0.2.1"],"cidrs":["2001:db8:2::/48","2001:db8:1::/48"],"ip_group_ids":[7,2,7]}`)},
		{ID: "allow", Type: RuleNodeAllow, Config: rawConfig(`{}`)},
		{ID: "block", Type: RuleNodeBlock, Config: rawConfig(`{"status_code":403}`)},
	}, Edges: []RuleEdge{
		{ID: "start-match", Source: "start", SourceHandle: "next", Target: "match"},
		{ID: "match-allow", Source: "match", SourceHandle: "true", Target: "allow"},
		{ID: "match-block", Source: "match", SourceHandle: "false", Target: "block"},
	}}
	before := cloneRuleGraph(t, graph)

	if _, err := CompileRuleGraph(graph); err != nil {
		t.Fatalf("CompileRuleGraph() error = %v", err)
	}
	if !reflect.DeepEqual(graph, before) {
		t.Fatalf("CompileRuleGraph mutated input:\ngot  %#v\nwant %#v", graph, before)
	}
}

func TestReferencedIPGroupIDs(t *testing.T) {
	graph := RuleGraph{Nodes: []RuleNode{
		{ID: "one", Type: RuleNodeIPMatch, Config: rawConfig(`{"ip_group_ids":[7,2,7]}`)},
		{ID: "geo", Type: RuleNodeGeoMatch, Config: rawConfig(`{"countries":["US"]}`)},
		{ID: "two", Type: RuleNodeIPMatch, Config: rawConfig(`{"ip_group_ids":[9,2]}`)},
		{ID: "bad", Type: RuleNodeIPMatch, Config: rawConfig(`{"ip_group_ids":`)},
	}}

	before := cloneRuleGraph(t, graph)
	if got, want := ReferencedIPGroupIDs(graph), []uint{2, 7, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ReferencedIPGroupIDs() = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(graph, before) {
		t.Fatalf("ReferencedIPGroupIDs mutated input:\ngot  %#v\nwant %#v", graph, before)
	}
}

func cloneRuleGraph(t *testing.T, graph RuleGraph) RuleGraph {
	t.Helper()
	clone := RuleGraph{
		SchemaVersion: graph.SchemaVersion,
		Nodes:         append([]RuleNode(nil), graph.Nodes...),
		Edges:         append([]RuleEdge(nil), graph.Edges...),
	}
	for i := range clone.Nodes {
		clone.Nodes[i].Config = append(json.RawMessage(nil), graph.Nodes[i].Config...)
	}
	return clone
}
