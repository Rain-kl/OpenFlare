// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"fmt"
	"sort"
)

// RuntimeRuleGraph is the compact graph loaded by the request runtime.
// Nodes are indexed by ID so execution does not scan the editor node list.
type RuntimeRuleGraph struct {
	Entry string                     `json:"entry"`
	Nodes map[string]RuntimeRuleNode `json:"nodes"`
}

// RuntimeRuleNode contains the typed runtime configuration and compiled exits
// for one graph node.
type RuntimeRuleNode struct {
	Type   RuleNodeType      `json:"type"`
	Config any               `json:"config"`
	Next   map[string]string `json:"next,omitempty"`
}

// CompileRuleGraph removes editor-only fields and compiles edges into per-node
// handle lookups. Callers should validate the graph before compiling it.
func CompileRuleGraph(graph RuleGraph) (RuntimeRuleGraph, error) {
	runtime := RuntimeRuleGraph{Nodes: make(map[string]RuntimeRuleNode, len(graph.Nodes))}
	for _, node := range graph.Nodes {
		config, err := compileRuleNodeConfig(node)
		if err != nil {
			return RuntimeRuleGraph{}, fmt.Errorf("节点 %s 的配置无法编译: %w", node.ID, err)
		}
		if node.Type == RuleNodeStart {
			if runtime.Entry != "" {
				return RuntimeRuleGraph{}, fmt.Errorf("规则图包含多个开始节点")
			}
			runtime.Entry = node.ID
		}
		runtime.Nodes[node.ID] = RuntimeRuleNode{Type: node.Type, Config: config}
	}
	if runtime.Entry == "" {
		return RuntimeRuleGraph{}, fmt.Errorf("规则图缺少开始节点")
	}
	for _, edge := range graph.Edges {
		node, ok := runtime.Nodes[edge.Source]
		if !ok {
			return RuntimeRuleGraph{}, fmt.Errorf("边 %s 的源节点 %s 不存在", edge.ID, edge.Source)
		}
		if _, ok := runtime.Nodes[edge.Target]; !ok {
			return RuntimeRuleGraph{}, fmt.Errorf("边 %s 的目标节点 %s 不存在", edge.ID, edge.Target)
		}
		if node.Next == nil {
			node.Next = make(map[string]string)
		}
		if _, exists := node.Next[edge.SourceHandle]; exists {
			return RuntimeRuleGraph{}, fmt.Errorf("节点 %s 的 %s 出口连接了多个目标", edge.Source, edge.SourceHandle)
		}
		node.Next[edge.SourceHandle] = edge.Target
		runtime.Nodes[edge.Source] = node
	}
	return runtime, nil
}

func compileRuleNodeConfig(node RuleNode) (any, error) {
	switch node.Type {
	case RuleNodeStart, RuleNodeAllow:
		var config struct{}
		return config, decodeStrictConfig(node.Config, &config)
	case RuleNodeIPMatch:
		var config IPMatchConfig
		if err := decodeStrictConfig(node.Config, &config); err != nil {
			return nil, err
		}
		config.IPs = sortedUniqueStrings(config.IPs)
		config.CIDRs = sortedUniqueStrings(config.CIDRs)
		config.IPGroupIDs = sortedUniqueUints(config.IPGroupIDs)
		return config, nil
	case RuleNodeGeoMatch:
		var config GeoMatchConfig
		if err := decodeStrictConfig(node.Config, &config); err != nil {
			return nil, err
		}
		config.Countries = sortedUniqueStrings(config.Countries)
		config.Regions = sortedUniqueStrings(config.Regions)
		return config, nil
	case RuleNodePoW:
		var config PoWNodeConfig
		return config, decodeStrictConfig(node.Config, &config)
	case RuleNodeBlock:
		var config BlockNodeConfig
		return config, decodeStrictConfig(node.Config, &config)
	default:
		return nil, fmt.Errorf("未知节点类型 %s", node.Type)
	}
}

// ReferencedIPGroupIDs returns the unique, sorted IP group IDs referenced by
// decodable IP match nodes. Graph validation reports malformed configurations.
func ReferencedIPGroupIDs(graph RuleGraph) []uint {
	ids := make([]uint, 0)
	for _, node := range graph.Nodes {
		if node.Type != RuleNodeIPMatch {
			continue
		}
		var config IPMatchConfig
		if err := decodeStrictConfig(node.Config, &config); err == nil {
			ids = append(ids, config.IPGroupIDs...)
		}
	}
	return sortedUniqueUints(ids)
}

func sortedUniqueStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	write := 0
	for _, value := range result {
		if write == 0 || result[write-1] != value {
			result[write] = value
			write++
		}
	}
	return result[:write]
}

func sortedUniqueUints(values []uint) []uint {
	result := append([]uint(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	write := 0
	for _, value := range result {
		if write == 0 || result[write-1] != value {
			result[write] = value
			write++
		}
	}
	return result[:write]
}
