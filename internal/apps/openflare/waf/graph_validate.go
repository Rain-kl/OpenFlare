// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strings"
)

const (
	maxRuleGraphNodes = 128
	maxRuleGraphEdges = 256
	maxRuleGraphBytes = 256 * 1024
)

var (
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)
	regionCodePattern  = regexp.MustCompile(`^[A-Z]{2}-[A-Z0-9]{1,3}$`)
)

func ValidateRuleGraph(ctx context.Context, graph RuleGraph, ipGroupExists func(context.Context, uint) (bool, error)) error {
	if graph.SchemaVersion != RuleGraphSchemaVersion {
		return fmt.Errorf("规则图 schema_version 必须为 %d", RuleGraphSchemaVersion)
	}
	if len(graph.Nodes) > maxRuleGraphNodes {
		return fmt.Errorf("规则图节点数不能超过 %d", maxRuleGraphNodes)
	}
	if len(graph.Edges) > maxRuleGraphEdges {
		return fmt.Errorf("规则图边数不能超过 %d", maxRuleGraphEdges)
	}
	if raw, err := json.Marshal(graph); err != nil {
		return fmt.Errorf("规则图无法序列化: %w", err)
	} else if len(raw) > maxRuleGraphBytes {
		return fmt.Errorf("规则图大小不能超过 256 KiB")
	}

	nodes := make(map[string]RuleNode, len(graph.Nodes))
	startCount, allowCount, startID := 0, 0, ""
	for _, node := range graph.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return errors.New("节点 ID 不能为空")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("节点 ID %s 重复", node.ID)
		}
		nodes[node.ID] = node
		switch node.Type {
		case RuleNodeStart:
			startCount++
			startID = node.ID
		case RuleNodeAllow:
			allowCount++
		case RuleNodeBlock, RuleNodeIPMatch, RuleNodeGeoMatch, RuleNodePoW:
		default:
			return fmt.Errorf("节点 %s 的类型 %s 未知", node.ID, node.Type)
		}
		if err := validateRuleNodeConfig(ctx, node, ipGroupExists); err != nil {
			return err
		}
	}
	if startCount != 1 {
		return errors.New("规则图必须恰好包含一个开始节点")
	}
	if allowCount != 1 {
		return errors.New("规则图必须恰好包含一个通过节点")
	}

	edgeIDs := make(map[string]struct{}, len(graph.Edges))
	outgoing := make(map[string][]RuleEdge)
	incoming := make(map[string]int)
	handleTargets := make(map[string]int)
	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.ID) == "" {
			return errors.New("边 ID 不能为空")
		}
		if _, exists := edgeIDs[edge.ID]; exists {
			return fmt.Errorf("边 ID %s 重复", edge.ID)
		}
		edgeIDs[edge.ID] = struct{}{}
		source, ok := nodes[edge.Source]
		if !ok {
			return fmt.Errorf("边 %s 的源节点 %s 不存在", edge.ID, edge.Source)
		}
		if _, ok := nodes[edge.Target]; !ok {
			return fmt.Errorf("边 %s 的目标节点 %s 不存在", edge.ID, edge.Target)
		}
		if !validSourceHandle(source.Type, edge.SourceHandle) {
			return fmt.Errorf("边 %s 的源端口 %s 不适用于节点 %s", edge.ID, edge.SourceHandle, edge.Source)
		}
		key := edge.Source + "\x00" + edge.SourceHandle
		handleTargets[key]++
		if handleTargets[key] > 1 {
			return fmt.Errorf("节点 %s 的 %s 出口连接了多个目标", edge.Source, edge.SourceHandle)
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
		incoming[edge.Target]++
	}
	if hasRuleGraphCycle(nodes, outgoing, incoming) {
		return errors.New("规则图不能包含循环")
	}
	for _, node := range graph.Nodes {
		for _, handle := range requiredHandles(node.Type) {
			if handleTargets[node.ID+"\x00"+handle] == 0 {
				return fmt.Errorf("节点 %s 的 %s 出口未连接", node.ID, handle)
			}
		}
	}

	reachable := walkRuleGraph(startID, outgoing)
	for _, node := range graph.Nodes {
		if !reachable[node.ID] {
			return fmt.Errorf("节点 %s 无法从开始节点到达", node.ID)
		}
	}
	for _, node := range graph.Nodes {
		if node.Type == RuleNodeStart && incoming[node.ID] != 0 {
			return fmt.Errorf("开始节点 %s 不能有入边", node.ID)
		}
		if node.Type != RuleNodeStart && incoming[node.ID] == 0 {
			return fmt.Errorf("节点 %s 必须至少有一条入边", node.ID)
		}
		if (node.Type == RuleNodeAllow || node.Type == RuleNodeBlock) && len(outgoing[node.ID]) != 0 {
			return fmt.Errorf("终止节点 %s 不能有出口", node.ID)
		}
	}
	if err := validateTerminalPaths(graph.Nodes, outgoing); err != nil {
		return err
	}
	return nil
}

func validateRuleNodeConfig(ctx context.Context, node RuleNode, exists func(context.Context, uint) (bool, error)) error {
	switch node.Type {
	case RuleNodeStart, RuleNodeAllow:
		var cfg struct{}
		if err := decodeStrictConfig(node.Config, &cfg); err != nil {
			return fmt.Errorf("节点 %s 的配置无效: %w", node.ID, err)
		}
	case RuleNodeIPMatch:
		var cfg IPMatchConfig
		if err := decodeStrictConfig(node.Config, &cfg); err != nil {
			return fmt.Errorf("节点 %s 的配置无效: %w", node.ID, err)
		}
		for _, raw := range cfg.IPs {
			if _, err := netip.ParseAddr(raw); err != nil {
				return fmt.Errorf("节点 %s 的 IP %s 无效", node.ID, raw)
			}
		}
		for _, raw := range cfg.CIDRs {
			if _, err := netip.ParsePrefix(raw); err != nil {
				return fmt.Errorf("节点 %s 的 CIDR %s 无效", node.ID, raw)
			}
		}
		for _, id := range cfg.IPGroupIDs {
			if id == 0 {
				return fmt.Errorf("节点 %s 引用的 IP 组 ID 无效", node.ID)
			}
			if exists == nil {
				return fmt.Errorf("节点 %s 无法校验 IP 组 %d", node.ID, id)
			}
			ok, err := exists(ctx, id)
			if err != nil {
				return fmt.Errorf("节点 %s 校验 IP 组 %d 失败: %w", node.ID, id, err)
			}
			if !ok {
				return fmt.Errorf("节点 %s 引用的 IP 组 %d 不存在", node.ID, id)
			}
		}
	case RuleNodeGeoMatch:
		var cfg GeoMatchConfig
		if err := decodeStrictConfig(node.Config, &cfg); err != nil {
			return fmt.Errorf("节点 %s 的配置无效: %w", node.ID, err)
		}
		for _, code := range cfg.Countries {
			if !countryCodePattern.MatchString(code) {
				return fmt.Errorf("节点 %s 的国家代码 %s 无效", node.ID, code)
			}
		}
		for _, code := range cfg.Regions {
			if !regionCodePattern.MatchString(code) {
				return fmt.Errorf("节点 %s 的地区代码 %s 无效", node.ID, code)
			}
		}
	case RuleNodePoW:
		var cfg PoWNodeConfig
		if err := decodeStrictConfig(node.Config, &cfg); err != nil {
			return fmt.Errorf("节点 %s 的配置无效: %w", node.ID, err)
		}
		if cfg.Difficulty < 1 || cfg.Difficulty > 16 {
			return fmt.Errorf("节点 %s 的 PoW 难度必须在 1-16 之间", node.ID)
		}
		if cfg.Algorithm != "fast" && cfg.Algorithm != "slow" {
			return fmt.Errorf("节点 %s 的 PoW 算法必须为 fast 或 slow", node.ID)
		}
		if cfg.SessionTTL < 60 {
			return fmt.Errorf("节点 %s 的 PoW 会话 TTL 不能小于 60 秒", node.ID)
		}
		if cfg.ChallengeTTL < 30 {
			return fmt.Errorf("节点 %s 的 PoW 挑战 TTL 不能小于 30 秒", node.ID)
		}
	case RuleNodeBlock:
		var cfg BlockNodeConfig
		if err := decodeStrictConfig(node.Config, &cfg); err != nil {
			return fmt.Errorf("节点 %s 的配置无效: %w", node.ID, err)
		}
		if cfg.StatusCode < 400 || cfg.StatusCode > 599 {
			return fmt.Errorf("节点 %s 的阻止状态码必须在 400-599 之间", node.ID)
		}
		if len([]byte(cfg.ResponseBody)) > maxWAFBlockBodyBytes {
			return fmt.Errorf("节点 %s 的阻止响应体不能超过 %d 字节", node.ID, maxWAFBlockBodyBytes)
		}
	}
	return nil
}

func decodeStrictConfig(raw json.RawMessage, dst any) error {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return errors.New("配置不能为 null")
	}
	if len(trimmed) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("配置包含额外 JSON 值")
	}
	return nil
}

func validSourceHandle(t RuleNodeType, handle string) bool {
	for _, expected := range requiredHandles(t) {
		if handle == expected {
			return true
		}
	}
	return false
}
func requiredHandles(t RuleNodeType) []string {
	switch t {
	case RuleNodeStart, RuleNodePoW:
		return []string{"next"}
	case RuleNodeIPMatch, RuleNodeGeoMatch:
		return []string{"true", "false"}
	default:
		return nil
	}
}
func walkRuleGraph(start string, outgoing map[string][]RuleEdge) map[string]bool {
	seen := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		for _, edge := range outgoing[id] {
			stack = append(stack, edge.Target)
		}
	}
	return seen
}
func hasRuleGraphCycle(nodes map[string]RuleNode, outgoing map[string][]RuleEdge, incoming map[string]int) bool {
	degree := make(map[string]int, len(nodes))
	queue := []string{}
	for id := range nodes {
		degree[id] = incoming[id]
		if degree[id] == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, edge := range outgoing[id] {
			degree[edge.Target]--
			if degree[edge.Target] == 0 {
				queue = append(queue, edge.Target)
			}
		}
	}
	return visited != len(nodes)
}
func validateTerminalPaths(nodes []RuleNode, outgoing map[string][]RuleEdge) error {
	reverse := map[string][]string{}
	stack := []string{}
	for _, node := range nodes {
		if node.Type == RuleNodeAllow || node.Type == RuleNodeBlock {
			stack = append(stack, node.ID)
		}
		for _, edge := range outgoing[node.ID] {
			reverse[edge.Target] = append(reverse[edge.Target], node.ID)
		}
	}
	terminating := map[string]bool{}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if terminating[id] {
			continue
		}
		terminating[id] = true
		stack = append(stack, reverse[id]...)
	}
	for _, node := range nodes {
		if !terminating[node.ID] {
			return fmt.Errorf("节点 %s 不存在通往终止节点的路径", node.ID)
		}
	}
	return nil
}
