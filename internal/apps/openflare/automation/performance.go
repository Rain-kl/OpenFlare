package automation

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/performance"
)

// PerformancePolicy identifies a saved policy and its effective values.
type PerformancePolicy struct {
	PolicyID string         `json:"policy_id"`
	Name     string         `json:"name"`
	Default  bool           `json:"is_default"`
	Settings map[string]any `json:"settings" swaggertype:"object"`
}

// PerformanceNode describes the policy assignment of an eligible edge node.
type PerformanceNode struct {
	NodeID   string `json:"node_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	PolicyID string `json:"policy_id,omitempty"`
}

// PerformanceParameter describes the permitted values of one tuning key.
type PerformanceParameter struct {
	Key          string   `json:"key"`
	Group        string   `json:"group"`
	ValueType    string   `json:"value_type"`
	DefaultValue string   `json:"default_value"`
	Min          *int     `json:"min,omitempty"`
	Max          *int     `json:"max,omitempty"`
	Choices      []string `json:"choices,omitempty"`
}

// PerformanceList contains policies, assigned nodes, and parameter definitions.
type PerformanceList struct {
	Policies   []PerformancePolicy    `json:"policies"`
	Nodes      []PerformanceNode      `json:"nodes"`
	Parameters []PerformanceParameter `json:"parameters"`
}

// PerformanceWriteResult identifies the tuning value saved on the controller.
type PerformanceWriteResult struct {
	PolicyID string `json:"policy_id"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	State    string `json:"state"`
}

// SetPerformanceBody updates one parameter.
type SetPerformanceBody struct {
	// Exact parameter key from openflare_performance_list, not a display label.
	Key string `json:"key" binding:"required" maxLength:"128"`
	// Text representation: a decimal integer within min/max, true/false for a boolean, or an exact choice. Follow the metadata returned by openflare_performance_list.
	Value string `json:"value" binding:"required" maxLength:"128"`
}

// @ID openflare_performance_list
// @Summary List OpenFlare Edge performance policies and parameter definitions
// @Description Read policy IDs, effective values, affected nodes, and parameter types, defaults, ranges or choices. Nodes without an explicit policy inherit the policy marked is_default.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} OperationResponse{data=PerformanceList}
// @Failure 401,403,404,500 {object} OperationResponse
// @x-automation {"cli":"performance list","scope":"performance:read","effect":"read"}
// @Router /api/v1/automation/performance [get]
func listPerformance(ctx context.Context, _ identity, _ map[string]string) (any, *operationError) {
	items, err := performance.ListPolicies(ctx)
	if err != nil {
		return nil, operationFailed(ctx, "list performance policies", err)
	}
	result := PerformanceList{Policies: make([]PerformancePolicy, 0, len(items.Policies)), Nodes: make([]PerformanceNode, 0, len(items.Nodes)), Parameters: make([]PerformanceParameter, 0, len(items.Parameters))}
	for _, item := range items.Policies {
		result.Policies = append(result.Policies, PerformancePolicy{PolicyID: decimalID(item.ID), Name: item.Name, Default: item.IsDefault, Settings: item.Settings})
	}
	for _, item := range items.Nodes {
		row := PerformanceNode{NodeID: item.NodeID, Name: item.Name, Status: item.Status}
		if item.PolicyID != nil {
			row.PolicyID = decimalID(*item.PolicyID)
		}
		result.Nodes = append(result.Nodes, row)
	}
	for _, item := range items.Parameters {
		valueType := "string"
		switch item.Default.(type) {
		case int:
			valueType = "integer"
		case bool:
			valueType = "boolean"
		}
		result.Parameters = append(result.Parameters, PerformanceParameter{Key: item.Key, Group: item.Group, ValueType: valueType, DefaultValue: fmt.Sprint(item.Default), Min: item.Min, Max: item.Max, Choices: item.Choices})
	}
	return result, nil
}

// @ID openflare_performance_set
// @Summary Change one OpenFlare Edge performance policy parameter
// @Description Update exactly one parameter of an existing policy and preserve all others. Use openflare_performance_list to identify the policy, affected nodes, and valid values. Editing the default policy affects every unassigned OpenFlare Edge node. The result confirms controller storage, not node application.
// @Tags automation
// @Accept json
// @Produce json
// @Security AutomationBearer
// @Param policy_id path string true "Performance policy record ID from openflare_performance_list"
// @Param body body SetPerformanceBody true "One parameter key and value"
// @Success 200 {object} OperationResponse{data=PerformanceWriteResult}
// @Failure 400,401,403,404,409,500 {object} OperationResponse
// @x-automation {"cli":"performance set","scope":"performance:write","effect":"write","parameters":{"policy_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/performance/{policy_id}/parameter [patch]
func setPerformance(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "policy_id")
	if err != nil {
		return nil, invalidArguments(err)
	}
	value, err := performance.SetParameter(ctx, id, input["key"], input["value"])
	if performance.IsInvalidPolicy(err) {
		return nil, invalidArguments(err)
	}
	if err != nil {
		return nil, resourceFailure(ctx, "set performance parameter", err)
	}
	return PerformanceWriteResult{PolicyID: decimalID(id), Key: input["key"], Value: fmt.Sprint(value), State: savedState}, nil
}
