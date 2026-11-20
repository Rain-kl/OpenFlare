// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package performance manages node performance policies and assignments.
package performance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/performance"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	errInvalidParams   = "Invalid performance policy parameters"
	errPolicyNotFound  = "Performance policy not found"
	errPolicyOperation = "Could not complete the performance policy operation"
	errInvalidNodes    = "All selected nodes must be Pingora edge nodes"
)

var errInvalidPolicy = errors.New("invalid performance policy parameters")

type policyRequest struct {
	Name     string                     `json:"name" binding:"required"`
	Settings map[string]json.RawMessage `json:"settings" swaggertype:"object"`
}

type assignmentRequest struct {
	NodeIDs []string `json:"node_ids" binding:"required,max=10000,dive,required,max=64"`
}

type policyView struct {
	ID       uint                 `json:"id"`
	Name     string               `json:"name"`
	Settings performance.Settings `json:"settings"`
}

type policyList struct {
	Policies   []policyView                  `json:"policies"`
	Nodes      []model.PerformancePolicyNode `json:"nodes"`
	Parameters []performance.Parameter       `json:"parameters"`
}

func listPolicies(ctx context.Context) (*policyList, error) {
	records, nodes, err := repository.ListPerformancePolicies(ctx)
	if err != nil {
		return nil, err
	}
	policies := make([]policyView, 0, len(records))
	for _, record := range records {
		settings, decodeErr := performance.Decode(record.SettingsJSON)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode performance policy %d: %w", record.ID, decodeErr)
		}
		policies = append(policies, policyView{ID: record.ID, Name: record.Name, Settings: settings})
	}
	return &policyList{Policies: policies, Nodes: nodes, Parameters: performance.Parameters}, nil
}

func savePolicy(ctx context.Context, id uint, request policyRequest) (*policyView, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, fmt.Errorf("%w: name must contain 1 to 100 characters", errInvalidPolicy)
	}
	settings, err := performance.Normalize(request.Settings)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidPolicy, err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	record := model.OpenFlarePerformancePolicy{ID: id, Name: name, SettingsJSON: string(encoded)}
	if id == 0 {
		err = repository.CreatePerformancePolicy(ctx, &record)
	} else {
		err = repository.UpdatePerformancePolicy(ctx, &record)
		if err == nil {
			websocket.NotifyAgentConfigChanged(ctx)
		}
	}
	if err != nil {
		return nil, err
	}
	return &policyView{ID: record.ID, Name: name, Settings: settings}, nil
}

func assignPolicy(ctx context.Context, id uint, nodeIDs []string) error {
	seen := make(map[string]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if _, exists := seen[nodeID]; exists {
			return repository.ErrInvalidPerformancePolicyNodes
		}
		seen[nodeID] = struct{}{}
	}
	if err := repository.AssignPerformancePolicy(ctx, id, nodeIDs); err != nil {
		return err
	}
	websocket.NotifyAgentConfigChanged(ctx)
	return nil
}
