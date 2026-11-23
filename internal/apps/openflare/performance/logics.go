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
	errInvalidNodes    = "All selected nodes must be OpenFlare Edge nodes"
	errDefaultPolicy   = "The default policy is inherited automatically"
)

var errInvalidPolicy = errors.New("invalid performance policy parameters")

// IsInvalidPolicy distinguishes validation errors from infrastructure failures.
func IsInvalidPolicy(err error) bool { return errors.Is(err, errInvalidPolicy) }

type policyRequest struct {
	Name     string                     `json:"name" binding:"required"`
	Settings map[string]json.RawMessage `json:"settings" swaggertype:"object"`
}

type assignmentRequest struct {
	NodeIDs []string `json:"node_ids" binding:"required,max=10000,dive,required,max=64"`
}

// PolicyView describes a performance policy and its resolved settings.
type PolicyView struct {
	ID        uint                 `json:"id"`
	Name      string               `json:"name"`
	IsDefault bool                 `json:"is_default"`
	Settings  performance.Settings `json:"settings"`
}

// PolicyList includes the saved policies, node assignments, and parameter definitions.
type PolicyList struct {
	Policies   []PolicyView                  `json:"policies"`
	Nodes      []model.PerformancePolicyNode `json:"nodes"`
	Parameters []performance.Parameter       `json:"parameters"`
}

// ListPolicies reads policy settings and current Pingora node assignments.
func ListPolicies(ctx context.Context) (*PolicyList, error) {
	records, nodes, err := repository.ListPerformancePolicies(ctx)
	if err != nil {
		return nil, err
	}
	policies := make([]PolicyView, 0, len(records))
	for _, record := range records {
		settings, decodeErr := performance.Decode(record.SettingsJSON)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode performance policy %d: %w", record.ID, decodeErr)
		}
		policies = append(policies, PolicyView{ID: record.ID, Name: record.Name, IsDefault: record.IsDefault(), Settings: settings})
	}
	return &PolicyList{Policies: policies, Nodes: nodes, Parameters: performance.Parameters}, nil
}

// SetParameter validates and updates one setting without replacing the policy.
func SetParameter(ctx context.Context, id uint, key, value string) (any, error) {
	var raw json.RawMessage
	found := false
	for _, parameter := range performance.Parameters {
		if parameter.Key != key {
			continue
		}
		found = true
		if _, isString := parameter.Default.(string); isString {
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			raw = encoded
		} else {
			raw = json.RawMessage(value)
		}
		break
	}
	if !found {
		return nil, fmt.Errorf("%w: use a parameter key from performance list", errInvalidPolicy)
	}
	var result any
	err := repository.EditPerformanceSettings(ctx, id, func(encoded string) (string, error) {
		var current map[string]json.RawMessage
		if err := json.Unmarshal([]byte(encoded), &current); err != nil {
			return "", err
		}
		if current == nil {
			current = make(map[string]json.RawMessage)
		}
		current[key] = raw
		settings, err := performance.Normalize(current)
		if err != nil {
			return "", fmt.Errorf("%w: %w", errInvalidPolicy, err)
		}
		result = settings[key]
		updated, err := json.Marshal(current)
		return string(updated), err
	})
	if err != nil {
		return nil, err
	}
	websocket.NotifyAgentConfigChanged(ctx)
	return result, nil
}

func savePolicy(ctx context.Context, id uint, request policyRequest) (*PolicyView, error) {
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
	return &PolicyView{ID: record.ID, Name: name, IsDefault: record.IsDefault(), Settings: settings}, nil
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
