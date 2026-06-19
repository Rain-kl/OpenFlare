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
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"

	exprlang "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

const maxWAFIPGroupSubscriptionBytes = 2 * 1024 * 1024

type ipGroupAutoRuleEnv struct {
	IP               string  `expr:"ip"`
	RequestCount     int     `expr:"request_count"`
	Status404Count   int     `expr:"status_404_count"`
	Status404Ratio   float64 `expr:"status_404_ratio"`
	IPHostCount      int     `expr:"ip_host_count"`
	IPHostRatio      float64 `expr:"ip_host_ratio"`
	ClientErrorCount int     `expr:"client_error_count"`
	ServerErrorCount int     `expr:"server_error_count"`
	LastSeenUnix     int64   `expr:"last_seen_unix"`
	statusCounts     map[int]int
}

func (env ipGroupAutoRuleEnv) StatusCount(code int) int {
	if env.statusCounts == nil {
		return 0
	}
	return env.statusCounts[code]
}

func (env ipGroupAutoRuleEnv) StatusRatio(code int) float64 {
	if env.RequestCount <= 0 || env.statusCounts == nil {
		return 0.0
	}
	return float64(env.statusCounts[code]) / float64(env.RequestCount)
}

type ipGroupAutoAccumulator struct {
	ip               string
	requestCount     int
	status404Count   int
	ipHostCount      int
	clientErrorCount int
	serverErrorCount int
	lastSeen         time.Time
	statusCounts     map[int]int
}

// SyncDueWAFIPGroups syncs all enabled automatic/subscription IP groups that are due.
func SyncDueWAFIPGroups(ctx context.Context) error {
	now := time.Now().UTC()
	groups, err := model.ListDueOpenFlareWAFIPGroups(ctx, now)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if _, err := syncOpenFlareWAFIPGroup(ctx, group, now); err != nil {
			continue
		}
	}
	return nil
}

func syncOpenFlareWAFIPGroup(ctx context.Context, group *model.OpenFlareWAFIPGroup, now time.Time) (*IPGroupSyncResult, error) {
	if group == nil {
		return nil, errors.New("IP 组不存在")
	}
	switch group.Type {
	case wafIPGroupTypeSubscription:
		return syncIPGroupSubscription(ctx, group, now)
	case wafIPGroupTypeAutomatic:
		return syncIPGroupAutomatic(ctx, group, now)
	default:
		return nil, errors.New("只有自动和订阅类型 IP 组支持同步")
	}
}

func syncIPGroupSubscription(ctx context.Context, group *model.OpenFlareWAFIPGroup, now time.Time) (*IPGroupSyncResult, error) {
	content, err := downloadIPGroupSubscription(group.SubscriptionURL)
	if err != nil {
		recordIPGroupSyncFailure(ctx, group, now, err)
		return nil, err
	}
	ips, err := parseIPGroupSubscription(content, group.SubscriptionFormat, group.SubscriptionMappingRule)
	if err != nil {
		recordIPGroupSyncFailure(ctx, group, now, err)
		return nil, err
	}
	ipListJSON, _ := json.Marshal(ips)
	nextSyncAt := now.Add(time.Duration(group.SyncIntervalMinutes) * time.Minute)
	group.IPList = string(ipListJSON)
	group.LastSyncedAt = &now
	group.NextSyncAt = &nextSyncAt
	group.LastSyncStatus = "success"
	group.LastSyncMessage = fmt.Sprintf("同步成功，共 %d 条 IP/IP 段", len(ips))
	if err := model.UpdateOpenFlareWAFIPGroupSyncResult(ctx, group); err != nil {
		return nil, err
	}
	broadcastIPGroupToAgents(ctx, group.ID)
	view, err := GetIPGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	return &IPGroupSyncResult{
		Group:      *view,
		IPCount:    len(ips),
		SyncedAt:   now.Format(time.RFC3339),
		NextSyncAt: nextSyncAt.Format(time.RFC3339),
		Status:     group.LastSyncStatus,
		Message:    group.LastSyncMessage,
	}, nil
}

func syncIPGroupAutomatic(ctx context.Context, group *model.OpenFlareWAFIPGroup, now time.Time) (*IPGroupSyncResult, error) {
	config, err := parseIPGroupAutoConfig(json.RawMessage(group.AutoConfig))
	if err != nil {
		recordIPGroupSyncFailure(ctx, group, now, err)
		return nil, err
	}

	var existingExtIPs []ipGroupExtIP
	if group.ExtIPs != "" && group.ExtIPs != "[]" {
		_ = json.Unmarshal([]byte(group.ExtIPs), &existingExtIPs)
	}

	activeExtIPs := make([]ipGroupExtIP, 0, len(existingExtIPs))
	for _, extIP := range existingExtIPs {
		if config.TTL > 0 {
			expirationTime := extIP.CapturedAt.Add(time.Duration(config.TTL) * time.Second)
			if expirationTime.Before(now) {
				continue
			}
		}
		activeExtIPs = append(activeExtIPs, extIP)
	}

	ips, err := evaluateParsedIPGroupAutoConfig(ctx, config, now)
	if err != nil {
		recordIPGroupSyncFailure(ctx, group, now, err)
		return nil, err
	}

	extIPMap := make(map[string]int)
	for idx, extIP := range activeExtIPs {
		extIPMap[extIP.IP] = idx
	}

	for _, ip := range ips {
		if idx, ok := extIPMap[ip]; ok {
			activeExtIPs[idx].CapturedAt = now
		} else {
			activeExtIPs = append(activeExtIPs, ipGroupExtIP{
				IP:         ip,
				CapturedAt: now,
			})
		}
	}

	finalIPs := make([]string, 0, len(activeExtIPs))
	for _, extIP := range activeExtIPs {
		finalIPs = append(finalIPs, extIP.IP)
	}
	finalIPs, err = normalizeIPList(finalIPs)
	if err != nil {
		recordIPGroupSyncFailure(ctx, group, now, err)
		return nil, err
	}

	extIPsJSON, _ := json.Marshal(activeExtIPs)
	ipListJSON, _ := json.Marshal(finalIPs)

	nextSyncAt := now.Add(time.Duration(normalizeIPGroupSyncInterval(group.SyncIntervalMinutes)) * time.Minute)
	group.IPList = string(ipListJSON)
	group.ExtIPs = string(extIPsJSON)
	group.LastSyncedAt = &now
	group.NextSyncAt = &nextSyncAt
	group.LastSyncStatus = "success"
	group.LastSyncMessage = fmt.Sprintf("自动规则执行成功，共命中 %d 个 IP，当前生效 %d 个 IP", len(ips), len(finalIPs))
	if err := model.UpdateOpenFlareWAFIPGroupSyncResult(ctx, group); err != nil {
		return nil, err
	}
	broadcastIPGroupToAgents(ctx, group.ID)
	view, err := GetIPGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	return &IPGroupSyncResult{
		Group:      *view,
		IPCount:    len(finalIPs),
		SyncedAt:   now.Format(time.RFC3339),
		NextSyncAt: nextSyncAt.Format(time.RFC3339),
		Status:     group.LastSyncStatus,
		Message:    group.LastSyncMessage,
	}, nil
}

func recordIPGroupSyncFailure(ctx context.Context, group *model.OpenFlareWAFIPGroup, now time.Time, syncErr error) {
	nextSyncAt := now.Add(time.Duration(normalizeIPGroupSyncInterval(group.SyncIntervalMinutes)) * time.Minute)
	group.LastSyncedAt = &now
	group.NextSyncAt = &nextSyncAt
	group.LastSyncStatus = "failed"
	group.LastSyncMessage = syncErr.Error()
	_ = model.UpdateOpenFlareWAFIPGroupSyncResult(ctx, group)
}

func evaluateParsedIPGroupAutoConfig(ctx context.Context, config ipGroupAutoConfig, now time.Time) ([]string, error) {
	if len(config.Rules) == 0 {
		return []string{}, nil
	}
	programs := make([]*vm.Program, 0, len(config.Rules))
	for i, rule := range config.Rules {
		program, err := exprlang.Compile(rule.Expr, exprlang.Env(ipGroupAutoRuleEnv{}), exprlang.AsBool())
		if err != nil {
			return nil, fmt.Errorf("自动规则 %s Expr 无效: %w", displayIPGroupAutoRuleName(rule, i), err)
		}
		programs = append(programs, program)
	}
	logs, err := model.ListOpenFlareAccessLogsForWAFIPGroup(ctx, model.OpenFlareAccessLogQuery{
		Since: now.Add(-time.Duration(config.LookbackMinutes) * time.Minute),
		Until: now,
	})
	if err != nil {
		return nil, err
	}
	accumulators := make(map[string]*ipGroupAutoAccumulator)
	for _, item := range logs {
		if item == nil {
			continue
		}
		ip, ok := normalizeIPLiteral(item.RemoteAddr)
		if !ok {
			continue
		}
		acc := accumulators[ip]
		if acc == nil {
			acc = &ipGroupAutoAccumulator{
				ip:           ip,
				statusCounts: make(map[int]int),
			}
			accumulators[ip] = acc
		}
		acc.requestCount++
		acc.statusCounts[item.StatusCode]++
		if item.StatusCode == http.StatusNotFound {
			acc.status404Count++
		}
		if item.StatusCode >= 400 && item.StatusCode < 500 {
			acc.clientErrorCount++
		}
		if item.StatusCode >= 500 {
			acc.serverErrorCount++
		}
		if hostIsIPLiteral(item.Host) {
			acc.ipHostCount++
		}
		if item.LoggedAt.After(acc.lastSeen) {
			acc.lastSeen = item.LoggedAt
		}
	}
	matched := make([]string, 0)
	for _, acc := range accumulators {
		env := acc.toExprEnv()
		for _, program := range programs {
			output, err := exprlang.Run(program, env)
			if err != nil {
				return nil, fmt.Errorf("执行自动规则失败: %w", err)
			}
			if matchedRule, ok := output.(bool); ok && matchedRule {
				matched = append(matched, acc.ip)
				break
			}
		}
	}
	return normalizeIPList(matched)
}

func (acc *ipGroupAutoAccumulator) toExprEnv() ipGroupAutoRuleEnv {
	env := ipGroupAutoRuleEnv{
		IP:               acc.ip,
		RequestCount:     acc.requestCount,
		Status404Count:   acc.status404Count,
		IPHostCount:      acc.ipHostCount,
		ClientErrorCount: acc.clientErrorCount,
		ServerErrorCount: acc.serverErrorCount,
		statusCounts:     acc.statusCounts,
	}
	if acc.requestCount > 0 {
		env.Status404Ratio = float64(acc.status404Count) / float64(acc.requestCount)
		env.IPHostRatio = float64(acc.ipHostCount) / float64(acc.requestCount)
	}
	if !acc.lastSeen.IsZero() {
		env.LastSeenUnix = acc.lastSeen.Unix()
	}
	return env
}

func displayIPGroupAutoRuleName(rule ipGroupAutoRule, index int) string {
	if rule.Name != "" {
		return rule.Name
	}
	return fmt.Sprintf("#%d", index+1)
}

func normalizeIPLiteral(value string) (string, bool) {
	host := strings.TrimSpace(value)
	if host == "" {
		return "", false
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	return addr.String(), true
}

func hostIsIPLiteral(value string) bool {
	_, ok := normalizeIPLiteral(value)
	return ok
}

func downloadIPGroupSubscription(rawURL string) ([]byte, error) {
	if err := validateSubscriptionURL(rawURL); err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("下载订阅失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("订阅返回状态码 %d", resp.StatusCode)
	}
	var buffer bytes.Buffer
	reader := io.LimitReader(resp.Body, maxWAFIPGroupSubscriptionBytes+1)
	if _, err := buffer.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("读取订阅内容失败: %w", err)
	}
	if buffer.Len() > maxWAFIPGroupSubscriptionBytes {
		return nil, fmt.Errorf("订阅内容不能超过 %d 字节", maxWAFIPGroupSubscriptionBytes)
	}
	return buffer.Bytes(), nil
}

func parseIPGroupSubscription(content []byte, format string, mappingRule string) ([]string, error) {
	switch normalizeIPGroupSubscriptionFormat(format) {
	case wafIPGroupSubscriptionFormatJSON:
		items, err := parseIPGroupJSONSubscription(content, mappingRule)
		if err != nil {
			return nil, err
		}
		return normalizeIPList(items)
	default:
		return normalizeIPList(parseIPGroupTextSubscription(string(content)))
	}
}

func parseIPGroupTextSubscription(text string) []string {
	lines := strings.Split(text, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item == "" || strings.HasPrefix(item, "#") {
			continue
		}
		items = append(items, item)
	}
	return items
}

func parseIPGroupJSONSubscription(content []byte, mappingRule string) ([]string, error) {
	var payload any
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, fmt.Errorf("JSON 订阅解析失败: %w", err)
	}
	nodes, err := selectJSONMappingNodes(payload, mappingRule)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(nodes))
	for _, node := range nodes {
		collectJSONStrings(node, &items)
	}
	if len(items) == 0 {
		return nil, errors.New("JSON 订阅没有解析到 IP/IP 段")
	}
	return items, nil
}

func selectJSONMappingNodes(payload any, mappingRule string) ([]any, error) {
	rule := strings.TrimSpace(mappingRule)
	if rule == "" || rule == "$" {
		return []any{payload}, nil
	}
	rule = strings.TrimPrefix(rule, "$.")
	nodes := []any{payload}
	for _, rawSegment := range strings.Split(rule, ".") {
		segment := strings.TrimSpace(rawSegment)
		if segment == "" {
			continue
		}
		expandArray := strings.HasSuffix(segment, "[]")
		segment = strings.TrimSuffix(segment, "[]")
		next := make([]any, 0)
		for _, node := range nodes {
			object, ok := node.(map[string]any)
			if !ok {
				continue
			}
			value, ok := object[segment]
			if !ok {
				continue
			}
			if expandArray {
				array, ok := value.([]any)
				if !ok {
					continue
				}
				next = append(next, array...)
			} else {
				next = append(next, value)
			}
		}
		nodes = next
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("JSON 映射规则 %q 未匹配到内容", mappingRule)
	}
	return nodes, nil
}

func collectJSONStrings(node any, items *[]string) {
	switch value := node.(type) {
	case string:
		*items = append(*items, value)
	case []any:
		for _, item := range value {
			collectJSONStrings(item, items)
		}
	}
}

func broadcastIPGroupToAgents(ctx context.Context, id uint) {
	groups, err := agent.WAFIPGroupsForAgent(ctx, []uint{id})
	if err != nil || len(groups) == 0 {
		if err != nil {
			slog.Debug("build waf ip group broadcast payload failed", "id", id, "error", err)
		}
		return
	}
	websocket.BroadcastWAFIPGroups(groups)
}
