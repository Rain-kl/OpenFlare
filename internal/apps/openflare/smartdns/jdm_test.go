// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"encoding/json"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestCompilePresetAndFlowGraphJDM(t *testing.T) {
	// Preset schema check (zen-engine 2.x expressionNode uses "key" and "value").
	presetJSON, err := CompilePresetJDM(model.DNSPolicyStrategySmart, PolicyConfig{
		Mode:          "smart",
		Weights:       PolicyWeights{Latency: 0.6, Load: 0.25, Geo: 0.15},
		UseTagWeights: true,
		Filters:       PolicyFilters{Tags: []string{"optimized"}},
	})
	require.NoError(t, err)

	var doc JDMDocument
	require.NoError(t, json.Unmarshal([]byte(presetJSON), &doc))
	require.Len(t, doc.Nodes, 3)
	assert.Contains(t, presetJSON, `"key":"mode"`)
	assert.Contains(t, presetJSON, `"key":"filters.tags"`)
	assert.NotContains(t, presetJSON, `"field":`)

	// Visual FlowGraph with switch branching -> action nodes -> output.
	graph := FlowGraph{
		Nodes: []FlowNode{
			{ID: "in", Type: "input", Label: "Input"},
			{
				ID:   "sw",
				Type: "condition",
				Data: FlowNodeData{
					HitPolicy: "first",
					Cases: []FlowSwitchCase{
						{ID: "cn_opt", Label: "China Telecom/Unicom", Condition: `client.country == "CN" && client.asn in [4134, 4809]`},
						{ID: "fallback", Label: "Default", Condition: ""},
					},
				},
			},
			{
				ID:   "act_cn",
				Type: "action",
				Data: FlowNodeData{
					Mode:          "smart",
					Weights:       &PolicyWeights{Latency: 0.7, Load: 0.2, Geo: 0.1},
					UseTagWeights: boolPtr(true),
					Filters:       &PolicyFilters{Tags: []string{"optimized"}},
					MaxAnswers:    intPtr(2),
				},
			},
			{
				ID:   "act_default",
				Type: "action",
				Data: FlowNodeData{
					Mode:          "load_balance",
					UseTagWeights: boolPtr(false),
				},
			},
			{ID: "out", Type: "output", Label: "Output"},
		},
		Edges: []FlowEdge{
			{ID: "e1", Source: "in", Target: "sw"},
			{ID: "e2", Source: "sw", SourceHandle: "cn_opt", Target: "act_cn"},
			{ID: "e3", Source: "sw", SourceHandle: "fallback", Target: "act_default"},
			{ID: "e4", Source: "act_cn", Target: "out"},
			{ID: "e5", Source: "act_default", Target: "out"},
		},
	}

	compiled, err := CompileFlowGraphToJDM(graph)
	require.NoError(t, err)

	base := PolicyDecision{Mode: "smart", MaxAnswers: 1, TTL: 60}
	cnRes, err := EvaluateJDM(compiled, base, EvalContext{Country: "CN", ASN: 4134, IPFamily: "ipv4", Domain: "app.example.com", QType: "A"})
	require.NoError(t, err)
	assert.Equal(t, "smart", cnRes.Mode)
	assert.InDelta(t, 0.7, cnRes.Weights.Latency, 1e-6)
	assert.Equal(t, []string{"optimized"}, cnRes.Filters.Tags)
	assert.Equal(t, 2, cnRes.MaxAnswers)
	assert.True(t, cnRes.UseTagWeights)

	usRes, err := EvaluateJDM(compiled, base, EvalContext{Country: "US", ASN: 15169, IPFamily: "ipv4", Domain: "app.example.com", QType: "A"})
	require.NoError(t, err)
	assert.Equal(t, "load_balance", usRes.Mode)
	assert.False(t, usRes.UseTagWeights)
}

func TestValidateJDMRejectsCyclesAndBrokenGraphs(t *testing.T) {
	cycle := `{"nodes":[{"id":"in","name":"In","type":"inputNode"},{"id":"a","name":"A","type":"expressionNode","content":{"expressions":[]}},{"id":"out","name":"Out","type":"outputNode"}],"edges":[{"id":"e1","sourceId":"in","targetId":"a"},{"id":"e2","sourceId":"a","targetId":"a"},{"id":"e3","sourceId":"a","targetId":"out"}]}`
	_, err := ValidateJDMJSON(cycle)
	require.Error(t, err)

	missingOutput := `{"nodes":[{"id":"in","name":"In","type":"inputNode"}],"edges":[]}`
	_, err = ValidateJDMJSON(missingOutput)
	require.Error(t, err)
}
