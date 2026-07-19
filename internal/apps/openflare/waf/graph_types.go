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
	// RuleNodeUACheck branches on User-Agent presence, classification, and lists.
	RuleNodeUACheck RuleNodeType = "ua_check"
	// RuleNodeSecurityCheck branches on basic request payload attack signatures.
	RuleNodeSecurityCheck RuleNodeType = "security_check"
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

// UACheckConfig configures User-Agent presence, whitelist, and block switches.
type UACheckConfig struct {
	RequireUA        bool     `json:"require_ua"`
	Browsers         []string `json:"browsers,omitempty"`
	OperatingSystems []string `json:"operating_systems,omitempty"`
	MatchMode        string   `json:"match_mode,omitempty"`
	BlockCommonBots  bool     `json:"block_common_bots"`
	BlockAbnormalUA  bool     `json:"block_abnormal_ua"`
	BlockCustomUA    bool     `json:"block_custom_ua"`
	CustomUAPatterns []string `json:"custom_ua_patterns,omitempty"`
}

// UA check match modes.
const (
	UACheckMatchModeAnd = "and"
	UACheckMatchModeOr  = "or"
)

// SecurityCheckConfig toggles basic payload signature protections.
// Default graph nodes enable path_traversal and file_inclusion only.
type SecurityCheckConfig struct {
	SQLInjection     bool `json:"sql_injection"`
	PathTraversal    bool `json:"path_traversal"`
	CommandInjection bool `json:"command_injection"`
	XSS              bool `json:"xss"`
	SSRF             bool `json:"ssrf"`
	FileInclusion    bool `json:"file_inclusion"`
	MaliciousUpload  bool `json:"malicious_upload"`
	XXE              bool `json:"xxe"`
	CRLFInjection    bool `json:"crlf_injection"`
}

// DefaultSecurityCheckConfig returns low false-positive defaults.
func DefaultSecurityCheckConfig() SecurityCheckConfig {
	return SecurityCheckConfig{
		PathTraversal: true,
		FileInclusion: true,
	}
}

// DefaultRuleGraph returns the minimal start-to-allow graph.
func DefaultRuleGraph() RuleGraph {
	return RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "start", Type: RuleNodeStart, Position: RulePosition{X: 0, Y: 0}, Config: json.RawMessage(`{}`)},
		{ID: "allow", Type: RuleNodeAllow, Position: RulePosition{X: 320, Y: 0}, Config: json.RawMessage(`{}`)},
	}, Edges: []RuleEdge{{ID: "start-allow", Source: "start", SourceHandle: "next", Target: "allow"}}}
}
