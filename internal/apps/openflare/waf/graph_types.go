// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import "encoding/json"

const RuleGraphSchemaVersion = 1

type RuleNodeType string

const (
	RuleNodeStart    RuleNodeType = "start"
	RuleNodeAllow    RuleNodeType = "allow"
	RuleNodeBlock    RuleNodeType = "block"
	RuleNodeIPMatch  RuleNodeType = "ip_match"
	RuleNodeGeoMatch RuleNodeType = "geo_match"
	RuleNodePoW      RuleNodeType = "pow"
)

type RuleGraph struct {
	SchemaVersion int        `json:"schema_version"`
	Nodes         []RuleNode `json:"nodes"`
	Edges         []RuleEdge `json:"edges"`
}

type RuleNode struct {
	ID       string          `json:"id"`
	Type     RuleNodeType    `json:"type"`
	Label    string          `json:"label,omitempty"`
	Position RulePosition    `json:"position"`
	Config   json.RawMessage `json:"config"`
}

type RulePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type RuleEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"source_handle"`
	Target       string `json:"target"`
}

type IPMatchConfig struct {
	IPs        []string `json:"ips,omitempty"`
	CIDRs      []string `json:"cidrs,omitempty"`
	IPGroupIDs []uint   `json:"ip_group_ids,omitempty"`
}

type GeoMatchConfig struct {
	Countries []string `json:"countries,omitempty"`
	Regions   []string `json:"regions,omitempty"`
}

type PoWNodeConfig struct {
	Algorithm    string `json:"algorithm"`
	Difficulty   int    `json:"difficulty"`
	SessionTTL   int    `json:"session_ttl"`
	ChallengeTTL int    `json:"challenge_ttl"`
}

type BlockNodeConfig struct {
	StatusCode   int    `json:"status_code"`
	ResponseBody string `json:"response_body,omitempty"`
}

func DefaultRuleGraph() RuleGraph {
	return RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "start", Type: RuleNodeStart, Position: RulePosition{X: 0, Y: 0}, Config: json.RawMessage(`{}`)},
		{ID: "allow", Type: RuleNodeAllow, Position: RulePosition{X: 320, Y: 0}, Config: json.RawMessage(`{}`)},
	}, Edges: []RuleEdge{{ID: "start-allow", Source: "start", SourceHandle: "next", Target: "allow"}}}
}
