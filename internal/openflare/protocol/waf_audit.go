// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

// WAFAudit describes the graph decision, matching rules, and optional request preview.
type WAFAudit struct {
	WAFAuditSummary
	Request *WAFRequestPreview `json:"request,omitempty"`
}

// WAFAuditSummary is the decision metadata shown in the attack log list.
type WAFAuditSummary struct {
	Action          string         `json:"action"`
	Reason          string         `json:"reason"`
	Method          string         `json:"method"`
	HTTPS           bool           `json:"https"`
	FlowID          uint           `json:"flow_id"`
	FlowName        string         `json:"flow_name"`
	FlowNodeID      string         `json:"flow_node_id"`
	AttackGroupID   uint           `json:"attack_group_id,omitempty"`
	AttackGroupName string         `json:"attack_group_name,omitempty"`
	Rules           []WAFRuleMatch `json:"rules,omitempty"`
}

// WAFRequestPreview contains a bounded reconstruction of the request as parsed by the edge.
type WAFRequestPreview struct {
	Message     string `json:"message"`
	Truncated   bool   `json:"truncated"`
	BodyOmitted bool   `json:"body_omitted"`
}

// WAFRuleMatch is the identifying metadata for a matching attack protection rule.
type WAFRuleMatch struct {
	ID       uint   `json:"id"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Level    string `json:"level"`
}
