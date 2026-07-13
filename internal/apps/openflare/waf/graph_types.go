// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import "encoding/json"

// RuleGraphSchemaVersion is the current persisted rule graph schema version.
const RuleGraphSchemaVersion = 1

// RuleNodeType identifies the behavior of a rule graph node.
type RuleNodeType string

const (
	// RuleNodeStart begins graph execution.
	RuleNodeStart RuleNodeType = "start"
	// RuleNodeAllow terminates execution with an allow decision.
	RuleNodeAllow RuleNodeType = "allow"
	// RuleNodeBlock terminates execution with a blocking response.
	RuleNodeBlock RuleNodeType = "block"
	// RuleNodeIPMatch branches on an IP match.
	RuleNodeIPMatch RuleNodeType = "ip_match"
	// RuleNodeGeoMatch branches on a geographic match.
	RuleNodeGeoMatch RuleNodeType = "geo_match"
	// RuleNodePoW runs a proof-of-work challenge before continuing.
	RuleNodePoW RuleNodeType = "pow"
)

// RuleGraph is the editor-facing representation of an executable WAF graph.
type RuleGraph struct {
	SchemaVersion int        `json:"schema_version"`
	Nodes         []RuleNode `json:"nodes"`
	Edges         []RuleEdge `json:"edges"`
}

// RuleNode stores one editor node and its type-specific configuration.
type RuleNode struct {
	ID       string          `json:"id"`
	Type     RuleNodeType    `json:"type"`
	Label    string          `json:"label,omitempty"`
	Position RulePosition    `json:"position"`
	Config   json.RawMessage `json:"config"`
}

// RulePosition stores a node's editor canvas coordinates.
type RulePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// RuleEdge connects one source handle to a target node.
type RuleEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"source_handle"`
	Target       string `json:"target"`
}

// IPMatchConfig configures literal, CIDR, and managed-group IP matching.
type IPMatchConfig struct {
	IPs        []string `json:"ips,omitempty"`
	CIDRs      []string `json:"cidrs,omitempty"`
	IPGroupIDs []uint   `json:"ip_group_ids,omitempty"`
}

// GeoMatchConfig configures country and region matching.
type GeoMatchConfig struct {
	Countries []string `json:"countries,omitempty"`
	Regions   []string `json:"regions,omitempty"`
}

// PoWNodeConfig configures a proof-of-work challenge node.
type PoWNodeConfig struct {
	Algorithm    string `json:"algorithm"`
	Difficulty   int    `json:"difficulty"`
	SessionTTL   int    `json:"session_ttl"`
	ChallengeTTL int    `json:"challenge_ttl"`
}

// BlockNodeConfig configures a terminal blocking response.
type BlockNodeConfig struct {
	StatusCode   int    `json:"status_code"`
	ResponseBody string `json:"response_body,omitempty"`
}

// DefaultRuleGraph returns the minimal start-to-allow graph.
func DefaultRuleGraph() RuleGraph {
	return RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "start", Type: RuleNodeStart, Position: RulePosition{X: 0, Y: 0}, Config: json.RawMessage(`{}`)},
		{ID: "allow", Type: RuleNodeAllow, Position: RulePosition{X: 320, Y: 0}, Config: json.RawMessage(`{}`)},
	}, Edges: []RuleEdge{{ID: "start-allow", Source: "start", SourceHandle: "next", Target: "allow"}}}
}
