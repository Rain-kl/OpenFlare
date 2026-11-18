// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package agent implements the OpenFlare agent protocol: node registration,
// heartbeat processing, access-log ingestion, and related middleware.
package agent

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"

	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
)

// 共享一个 GeoIP 服务实例：mmdb 打开（mmap + 解析元数据）成本不低，缺文件时还会
// 同步下载，绝不能每个上报批次重建。maxminddb.Reader 并发安全，无需额外加锁。
// 初始化失败不锁存：下一批上报会重试（与旧行为一致）。
var (
	sharedAccessLogGeoMu       sync.Mutex
	sharedAccessLogGeoInstance pkggeoip.Service
)

func sharedAccessLogGeoService(ctx context.Context) pkggeoip.Service {
	sharedAccessLogGeoMu.Lock()
	defer sharedAccessLogGeoMu.Unlock()
	if sharedAccessLogGeoInstance == nil {
		mmdbPath, err := ofgeoip.EnsureServerMMDB()
		if err != nil {
			slog.WarnContext(ctx, "seed access log geo database failed", "error", err)
		}
		service, err := pkggeoip.NewMaxMindGeoIPServiceWithContext(ctx, mmdbPath, "")
		if err != nil {
			slog.WarnContext(ctx, "initialize access log geo service failed", "error", err)
			return nil
		}
		sharedAccessLogGeoInstance = service
	}
	return sharedAccessLogGeoInstance
}

// resolveAccessLogRegion resolves the region name for an access-log remote address.
// mmdb Lookup 本身是内存映射 trie 查找（微秒级），无需再建应用层 IP 缓存。
// resolveAccessLogRegion resolves the region name for an access-log remote address.
// mmdb Lookup 本身是内存映射 trie 查找（微秒级），无需再建应用层 IP 缓存。
func resolveAccessLogRegion(ctx context.Context, rawIP string) string {
	normalizedIP := normalizeAccessLogIP(rawIP)
	if normalizedIP == "" {
		return ""
	}
	service := sharedAccessLogGeoService(ctx)
	if service == nil {
		return ""
	}
	info, err := service.GetGeoInfo(net.ParseIP(normalizedIP))
	if err != nil || info == nil {
		return ""
	}
	region := strings.TrimSpace(info.Name)
	if region == "" {
		region = strings.TrimSpace(info.ISOCode)
	}
	return region
}

func normalizeAccessLogIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}

	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}
