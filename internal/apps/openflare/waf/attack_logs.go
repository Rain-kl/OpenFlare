// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
)

const attackEventWindowSeconds = 300

type attackLogQuery struct {
	Page       int    `form:"page" binding:"min=0,max=1000000"` // Zero-based page
	PageSize   int    `form:"page_size" binding:"min=0,max=100"`
	RemoteAddr string `form:"remote_addr" binding:"max=128"`
	Host       string `form:"host" binding:"max=255"`
	NodeID     string `form:"node_id" binding:"max=64"`
	Action     string `form:"action" binding:"omitempty,oneof=all blocked observed"`
	Since      int64  `form:"since" binding:"min=0"` // Unix seconds, inclusive
	Until      int64  `form:"until" binding:"min=0"` // Unix seconds, exclusive
	Exact      bool   `form:"exact"`                 // Exact host and source IP
}

type attackLogEntry struct {
	ID         uint64    `json:"id,string"`
	NodeID     string    `json:"node_id"`
	LoggedAt   time.Time `json:"logged_at"`
	RemoteAddr string    `json:"remote_addr"`
	Region     string    `json:"region"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	protocol.WAFAudit
}

type attackLogPage[T any] struct {
	Total   int64 `json:"total"`
	Results []T   `json:"results"`
}

func (input attackLogQuery) filter(defaultAction string, now time.Time) (model.OpenFlareAccessLogQuery, error) {
	query := model.OpenFlareAccessLogQuery{
		RemoteAddr: strings.TrimSpace(input.RemoteAddr), Host: strings.ToLower(strings.TrimSpace(input.Host)),
		NodeID: input.NodeID, Page: input.Page, PageSize: input.PageSize, WAFAction: input.Action,
		ExactRemoteAddr: input.Exact, Until: now, Since: now.Add(-24 * time.Hour),
	}
	if input.Since != 0 {
		query.Since = time.Unix(input.Since, 0).UTC()
	}
	if input.Until != 0 {
		query.Until = time.Unix(input.Until, 0).UTC()
		if input.Since == 0 {
			query.Since = query.Until.Add(-24 * time.Hour)
		}
	}
	if !query.Since.Before(query.Until) {
		return query, errors.New(errAttackLogFilters)
	}
	if input.Exact && query.Host != "" {
		query.Hosts = []string{query.Host}
	}
	if query.WAFAction == "" {
		query.WAFAction = defaultAction
	}
	if query.PageSize == 0 {
		query.PageSize = 25
	}
	return query, nil
}

func listAttackLogs(ctx context.Context, query model.OpenFlareAccessLogQuery) (attackLogPage[attackLogEntry], error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return attackLogPage[attackLogEntry]{}, err
	}
	total, _, _, err := store.AccessLogs.Count(ctx, query)
	if err != nil {
		return attackLogPage[attackLogEntry]{}, err
	}
	rows, err := store.AccessLogs.List(ctx, query)
	if err != nil {
		return attackLogPage[attackLogEntry]{}, err
	}
	result := attackLogPage[attackLogEntry]{Total: total, Results: make([]attackLogEntry, 0, len(rows))}
	for _, row := range rows {
		var audit protocol.WAFAudit
		if err := json.Unmarshal([]byte(row.WAFAudit), &audit); err != nil {
			return attackLogPage[attackLogEntry]{}, err
		}
		result.Results = append(result.Results, attackLogEntry{
			ID: row.ID, NodeID: row.NodeID, LoggedAt: row.LoggedAt, RemoteAddr: row.RemoteAddr,
			Region: row.Region, Host: row.Host, Path: row.Path, StatusCode: row.StatusCode, WAFAudit: audit,
		})
	}
	return result, nil
}

func listAttackEvents(ctx context.Context, query model.OpenFlareAccessLogQuery) (attackLogPage[model.OpenFlareAttackEvent], error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return attackLogPage[model.OpenFlareAttackEvent]{}, err
	}
	rows, total, err := store.AccessLogs.AttackEvents(ctx, query, attackEventWindowSeconds)
	return attackLogPage[model.OpenFlareAttackEvent]{Total: total, Results: rows}, err
}
