// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

// WAFAudit describes the graph decision and the matching rules without captured payloads.
type WAFAudit struct {
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

// WAFRuleMatch is the identifying metadata for a matching attack protection rule.
type WAFRuleMatch struct {
	ID       uint   `json:"id"`
	Category string `json:"category"`
	Message  string `json:"message"`
	Level    string `json:"level"`
}
