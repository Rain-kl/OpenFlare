// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
)

type customHeaderInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func decodeStoredUpstreams(raw string, fallbackOriginURL string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return normalizeUpstreams(fallbackOriginURL, nil)
	}
	var upstreams []string
	if err := json.Unmarshal([]byte(text), &upstreams); err != nil {
		return nil, errors.New("upstreams payload is invalid")
	}
	return normalizeUpstreams(fallbackOriginURL, upstreams)
}

func normalizeUpstreams(originURL string, upstreams []string) ([]string, error) {
	candidates := upstreams
	if len(candidates) == 0 {
		candidates = []string{originURL}
	}
	normalized := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil, errors.New("upstream is required")
	}
	return normalized, nil
}

func decodeStoredCustomHeaders(raw string) ([]customHeaderInput, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []customHeaderInput{}, nil
	}
	var headers []customHeaderInput
	if err := json.Unmarshal([]byte(text), &headers); err != nil {
		return nil, errors.New("custom_headers payload is invalid")
	}
	return headers, nil
}

func decodeStoredCacheRules(raw string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []string{}, nil
	}
	var rules []string
	if err := json.Unmarshal([]byte(text), &rules); err != nil {
		return nil, errors.New("cache_rules payload is invalid")
	}
	normalized := make([]string, 0, len(rules))
	for _, rule := range rules {
		item := strings.TrimSpace(rule)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func normalizeUpstreamType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "tunnel", "pages":
		return value
	default:
		return "direct"
	}
}

func normalizeTunnelTargetProtocol(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "http", "https", "tcp":
		return value
	default:
		return "http"
	}
}

func normalizePEM(content string) string {
	return strings.TrimSpace(content) + "\n"
}

func certificateCertFileName(id uint) string {
	return fmt.Sprintf("%d.crt", id)
}

func certificateKeyFileName(id uint) string {
	return fmt.Sprintf("%d.key", id)
}

func dedupeSupportFiles(files []SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	unique := make(map[string]SupportFile, len(files))
	for _, file := range files {
		unique[file.Path] = file
	}
	result := make([]SupportFile, 0, len(unique))
	for _, file := range unique {
		result = append(result, file)
	}
	return result
}

func relayAgentAddress(node *model.OpenFlareNode) string {
	if node == nil {
		return ""
	}
	port := node.RelayVhostHTTPPort
	if port <= 0 {
		port = 8080
	}
	addr := strings.TrimSpace(node.RelayAgentAccessAddr)
	if addr == "" {
		addr = strings.TrimSpace(node.RelayClientAccessAddr)
	}
	if addr == "" {
		addr = strings.TrimSpace(node.IP)
	}
	if addr == "" {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	if strings.Contains(addr, ":") && strings.Count(addr, ":") > 1 {
		return net.JoinHostPort(addr, strconv.Itoa(port))
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func resolveTunnelEdgeUpstreamURL(ctx context.Context) (string, error) {
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return "", err
	}
	for index := range nodes {
		node := &nodes[index]
		if node.NodeType == "tunnel_relay" {
			if addr := relayAgentAddress(node); addr != "" {
				return "http://" + addr, nil
			}
		}
	}
	return "http://127.0.0.1:8080", nil
}

func listWAFIPGroupsByIDs(ctx context.Context, ids []uint) ([]*model.OpenFlareWAFIPGroup, error) {
	if len(ids) == 0 {
		return []*model.OpenFlareWAFIPGroup{}, nil
	}
	groups := make([]*model.OpenFlareWAFIPGroup, 0, len(ids))
	for _, id := range ids {
		group, err := repository.GetOpenFlareWAFIPGroupByID(ctx, id)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, nil
}
