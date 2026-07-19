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
	maxRuleGraphNodes       = 128
	maxRuleGraphEdges       = 256
	maxRuleGraphBytes       = 256 * 1024
	maxUACustomPatterns     = 32
	maxUACustomPatternBytes = 256
)

var (
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)
	regionCodePattern  = regexp.MustCompile(`^[A-Z]{2}-[A-Z0-9]{1,3}$`)
)

// ValidateRuleGraph validates graph structure, node configuration, references,
// reachability, and termination before compilation.
func ValidateRuleGraph(ctx context.Context, graph RuleGraph, ipGroupExists func(context.Context, uint) (bool, error)) error {
	if err := validateRuleGraphLimits(graph); err != nil {
		return err
	}
	nodes, startID, err := validateRuleGraphNodes(ctx, graph.Nodes, ipGroupExists)
	if err != nil {
		return err
	}
	outgoing, incoming, handleTargets, err := validateRuleGraphEdges(nodes, graph.Edges)
	if err != nil {
		return err
	}
	if hasRuleGraphCycle(nodes, outgoing, incoming) {
		return errors.New("规则图不能包含循环")
	}
	if err := validateRequiredHandles(graph.Nodes, handleTargets); err != nil {
		return err
	}
	if err := validateRuleGraphConnectivity(graph.Nodes, startID, outgoing, incoming); err != nil {
		return err
	}
	return validateTerminalPaths(graph.Nodes, outgoing)
}

func validateRuleGraphLimits(graph RuleGraph) error {
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
	return nil
}

func validateRuleGraphNodes(ctx context.Context, graphNodes []RuleNode, ipGroupExists func(context.Context, uint) (bool, error)) (map[string]RuleNode, string, error) {
	nodes := make(map[string]RuleNode, len(graphNodes))
	startCount, allowCount, startID := 0, 0, ""
	for _, node := range graphNodes {
		if strings.TrimSpace(node.ID) == "" {
			return nil, "", errors.New("节点 ID 不能为空")
		}
		if _, exists := nodes[node.ID]; exists {
			return nil, "", fmt.Errorf("节点 ID %s 重复", node.ID)
		}
		nodes[node.ID] = node
		switch node.Type {
		case RuleNodeStart:
			startCount++
			startID = node.ID
		case RuleNodeAllow:
			allowCount++
		case RuleNodeBlock, RuleNodeIPMatch, RuleNodeGeoMatch, RuleNodePoW, RuleNodeUACheck, RuleNodeSecurityCheck:
		default:
			return nil, "", fmt.Errorf("节点 %s 的类型 %s 未知", node.ID, node.Type)
		}
		if err := validateRuleNodeConfig(ctx, node, ipGroupExists); err != nil {
			return nil, "", err
		}
	}
	if startCount != 1 {
		return nil, "", errors.New("规则图必须恰好包含一个开始节点")
	}
	if allowCount != 1 {
		return nil, "", errors.New("规则图必须恰好包含一个通过节点")
	}
	return nodes, startID, nil
}

func validateRuleGraphEdges(nodes map[string]RuleNode, graphEdges []RuleEdge) (map[string][]RuleEdge, map[string]int, map[string]int, error) {
	edgeIDs := make(map[string]struct{}, len(graphEdges))
	outgoing := make(map[string][]RuleEdge)
	incoming := make(map[string]int)
	handleTargets := make(map[string]int)
	for _, edge := range graphEdges {
		if strings.TrimSpace(edge.ID) == "" {
			return nil, nil, nil, errors.New("边 ID 不能为空")
		}
		if _, exists := edgeIDs[edge.ID]; exists {
			return nil, nil, nil, fmt.Errorf("边 ID %s 重复", edge.ID)
		}
		edgeIDs[edge.ID] = struct{}{}
		source, ok := nodes[edge.Source]
		if !ok {
			return nil, nil, nil, fmt.Errorf("边 %s 的源节点 %s 不存在", edge.ID, edge.Source)
		}
		if _, ok := nodes[edge.Target]; !ok {
			return nil, nil, nil, fmt.Errorf("边 %s 的目标节点 %s 不存在", edge.ID, edge.Target)
		}
		if !validSourceHandle(source.Type, edge.SourceHandle) {
			return nil, nil, nil, fmt.Errorf("边 %s 的源端口 %s 不适用于节点 %s", edge.ID, edge.SourceHandle, edge.Source)
		}
		key := edge.Source + "\x00" + edge.SourceHandle
		handleTargets[key]++
		if handleTargets[key] > 1 {
			return nil, nil, nil, fmt.Errorf("节点 %s 的 %s 出口连接了多个目标", edge.Source, edge.SourceHandle)
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
		incoming[edge.Target]++
	}
	return outgoing, incoming, handleTargets, nil
}

func validateRequiredHandles(nodes []RuleNode, handleTargets map[string]int) error {
	for _, node := range nodes {
		for _, handle := range requiredHandles(node.Type) {
			if handleTargets[node.ID+"\x00"+handle] == 0 {
				return fmt.Errorf("节点 %s 的 %s 出口未连接", node.ID, handle)
			}
		}
	}
	return nil
}

func validateRuleGraphConnectivity(nodes []RuleNode, startID string, outgoing map[string][]RuleEdge, incoming map[string]int) error {
	reachable := walkRuleGraph(startID, outgoing)
	for _, node := range nodes {
		if !reachable[node.ID] {
			return fmt.Errorf("节点 %s 无法从开始节点到达", node.ID)
		}
	}
	for _, node := range nodes {
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
	return nil
}

func validateRuleNodeConfig(ctx context.Context, node RuleNode, exists func(context.Context, uint) (bool, error)) error {
	switch node.Type {
	case RuleNodeStart, RuleNodeAllow:
		return validateEmptyNodeConfig(node)
	case RuleNodeIPMatch:
		return validateIPMatchNodeConfig(ctx, node, exists)
	case RuleNodeGeoMatch:
		return validateGeoMatchNodeConfig(node)
	case RuleNodePoW:
		return validatePoWNodeConfig(node)
	case RuleNodeUACheck:
		return validateUACheckNodeConfig(node)
	case RuleNodeSecurityCheck:
		return validateSecurityCheckNodeConfig(node)
	case RuleNodeBlock:
		return validateBlockNodeConfig(node)
	}
	return nil
}

func validateEmptyNodeConfig(node RuleNode) error {
	var cfg struct{}
	return decodeNodeConfig(node, &cfg)
}

func validateIPMatchNodeConfig(ctx context.Context, node RuleNode, exists func(context.Context, uint) (bool, error)) error {
	var cfg IPMatchConfig
	if err := decodeNodeConfig(node, &cfg); err != nil {
		return err
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
		if err := validateIPGroupReference(ctx, node.ID, id, exists); err != nil {
			return err
		}
	}
	return nil
}

func validateIPGroupReference(ctx context.Context, nodeID string, id uint, exists func(context.Context, uint) (bool, error)) error {
	if id == 0 {
		return fmt.Errorf("节点 %s 引用的 IP 组 ID 无效", nodeID)
	}
	if exists == nil {
		return fmt.Errorf("节点 %s 无法校验 IP 组 %d", nodeID, id)
	}
	ok, err := exists(ctx, id)
	if err != nil {
		return fmt.Errorf("节点 %s 校验 IP 组 %d 失败: %w", nodeID, id, err)
	}
	if !ok {
		return fmt.Errorf("节点 %s 引用的 IP 组 %d 不存在", nodeID, id)
	}
	return nil
}

func validateGeoMatchNodeConfig(node RuleNode) error {
	var cfg GeoMatchConfig
	if err := decodeNodeConfig(node, &cfg); err != nil {
		return err
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
	return nil
}

func validatePoWNodeConfig(node RuleNode) error {
	var cfg PoWNodeConfig
	if err := decodeNodeConfig(node, &cfg); err != nil {
		return err
	}
	if cfg.Difficulty < 1 || cfg.Difficulty > 16 {
		return fmt.Errorf("节点 %s 的 PoW 难度必须在 1-16 之间", node.ID)
	}
	if cfg.Algorithm != powAlgorithmFast && cfg.Algorithm != powAlgorithmSlow {
		return fmt.Errorf("节点 %s 的 PoW 算法必须为 fast 或 slow", node.ID)
	}
	if cfg.SessionTTL < minPoWSessionTTLSeconds {
		return fmt.Errorf("节点 %s 的 PoW 会话 TTL 不能小于 60 秒", node.ID)
	}
	if cfg.ChallengeTTL < minPoWChallengeTTLSeconds {
		return fmt.Errorf("节点 %s 的 PoW 挑战 TTL 不能小于 30 秒", node.ID)
	}
	return nil
}

func validateBlockNodeConfig(node RuleNode) error {
	var cfg BlockNodeConfig
	if err := decodeNodeConfig(node, &cfg); err != nil {
		return err
	}
	if cfg.StatusCode < 400 || cfg.StatusCode > 599 {
		return fmt.Errorf("节点 %s 的阻止状态码必须在 400-599 之间", node.ID)
	}
	if len([]byte(cfg.ResponseBody)) > maxWAFBlockBodyBytes {
		return fmt.Errorf("节点 %s 的阻止响应体不能超过 %d 字节", node.ID, maxWAFBlockBodyBytes)
	}
	return nil
}

func validateUACheckNodeConfig(node RuleNode) error {
	var cfg UACheckConfig
	if err := decodeNodeConfig(node, &cfg); err != nil {
		return err
	}
	mode := cfg.MatchMode
	if mode == "" {
		mode = UACheckMatchModeOr
	}
	if mode != UACheckMatchModeAnd && mode != UACheckMatchModeOr {
		return fmt.Errorf("节点 %s 的匹配模式必须为 and 或 or", node.ID)
	}
	for _, label := range cfg.Browsers {
		if !uaBrowserLabels[label] {
			return fmt.Errorf("节点 %s 的浏览器标签 %s 无效", node.ID, label)
		}
	}
	for _, label := range cfg.OperatingSystems {
		if !uaOSLabels[label] {
			return fmt.Errorf("节点 %s 的操作系统标签 %s 无效", node.ID, label)
		}
	}
	if len(cfg.CustomUAPatterns) > maxUACustomPatterns {
		return fmt.Errorf("节点 %s 的自定义 UA 正则不能超过 %d 条", node.ID, maxUACustomPatterns)
	}
	for _, pattern := range cfg.CustomUAPatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("节点 %s 的自定义 UA 正则不能为空", node.ID)
		}
		if len(pattern) > maxUACustomPatternBytes {
			return fmt.Errorf("节点 %s 的自定义 UA 正则不能超过 %d 字节", node.ID, maxUACustomPatternBytes)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("节点 %s 的自定义 UA 正则无效: %s", node.ID, pattern)
		}
	}
	if cfg.BlockCustomUA && len(cfg.CustomUAPatterns) == 0 {
		return fmt.Errorf("节点 %s 开启屏蔽自定义 UA 时至少需要一条正则", node.ID)
	}
	return nil
}

func validateSecurityCheckNodeConfig(node RuleNode) error {
	var cfg SecurityCheckConfig
	return decodeNodeConfig(node, &cfg)
}

var uaBrowserLabels = map[string]bool{
	"Chrome": true, "Safari": true, "Firefox": true, "Edge": true, "Opera": true,
	"Chromium": true, "WeChat": true, "Postman": true, "CLI": true, "Bot": true,
	"Unknown": true, "Other": true,
}

var uaOSLabels = map[string]bool{
	"Android": true, "iOS": true, "Windows": true, "macOS": true, "Chrome OS": true,
	"Linux": true, "Bot": true, "Unknown": true, "Other": true,
}

func decodeNodeConfig(node RuleNode, dst any) error {
	if err := decodeStrictConfig(node.Config, dst); err != nil {
		return fmt.Errorf("节点 %s 的配置无效: %w", node.ID, err)
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
	case RuleNodeIPMatch, RuleNodeGeoMatch, RuleNodeUACheck, RuleNodeSecurityCheck:
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
