// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
)

const (
	wafAuditReasonLength    = 48
	wafAuditMethodLength    = 24
	wafAuditFlowLength      = 240
	wafAuditNodeIDLength    = 128
	wafAuditGroupNameLength = 100
	wafAuditRuleLimit       = 16
	wafAuditCategoryLength  = 48
	wafAuditMessageLength   = 160
	wafAuditLevelLength     = 16
	wafAuditPathLength      = 2048
	wafRequestPreviewBytes  = 8 * 1024
)

func serializeWAFAudit(audit *protocol.WAFAudit) (string, string, error) {
	if audit == nil || (audit.Action != "blocked" && audit.Action != "observed") {
		return "", "", nil
	}
	value := *audit
	value.Reason = truncateForDatabase(value.Reason, wafAuditReasonLength)
	value.Method = truncateForDatabase(value.Method, wafAuditMethodLength)
	value.FlowName = truncateForDatabase(value.FlowName, wafAuditFlowLength)
	value.FlowNodeID = truncateForDatabase(value.FlowNodeID, wafAuditNodeIDLength)
	value.AttackGroupName = truncateForDatabase(value.AttackGroupName, wafAuditGroupNameLength)
	if value.Request != nil && len(value.Request.Message) > wafRequestPreviewBytes {
		preview := *value.Request
		end := wafRequestPreviewBytes
		for !utf8.RuneStart(preview.Message[end]) {
			end--
		}
		preview.Message = preview.Message[:end]
		preview.Truncated = true
		value.Request = &preview
	}
	rules := audit.Rules[:min(len(audit.Rules), wafAuditRuleLimit)]
	value.Rules = make([]protocol.WAFRuleMatch, 0, len(rules))
	for _, entry := range rules {
		entry.Category = truncateForDatabase(entry.Category, wafAuditCategoryLength)
		entry.Message = truncateForDatabase(entry.Message, wafAuditMessageLength)
		entry.Level = truncateForDatabase(entry.Level, wafAuditLevelLength)
		value.Rules = append(value.Rules, entry)
	}
	data, err := json.Marshal(value)
	return value.Action, string(data), err
}

func attackRequestPath(uri string) string {
	path, _, _ := strings.Cut(uri, "?")
	return truncateForDatabase(path, wafAuditPathLength)
}
