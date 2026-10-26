// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	defaultNodePageSize = 20
	maxNodePageSize     = 100
	maxNodeSearchLength = 256
)

// ListQuery filters the bounded node list.
type ListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Search   string `form:"search"`
	Status   string `form:"status"`
	NodeType string `form:"node_type"`
}

// ListResult contains one node page and its filtered total.
type ListResult struct {
	Total    int64   `json:"total"`
	Results  []*View `json:"results"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

func normalizeListQuery(query ListQuery) (ListQuery, error) {
	if query.Page < 0 || query.PageSize < 0 {
		return query, errors.New(errInvalidNodeListQuery)
	}
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = defaultNodePageSize
	}
	if query.PageSize > maxNodePageSize {
		query.PageSize = maxNodePageSize
	}
	if query.Page-1 > int(^uint(0)>>1)/query.PageSize {
		return query, errors.New(errInvalidNodeListQuery)
	}
	query.Search = strings.TrimSpace(query.Search)
	if len(query.Search) > maxNodeSearchLength {
		return query, errors.New(errInvalidNodeListQuery)
	}
	switch query.Status {
	case "", nodeStatusPending, nodeStatusOnline, nodeStatusOffline:
	default:
		return query, errors.New(errInvalidNodeListQuery)
	}
	switch query.NodeType {
	case "", nodeTypeEdgeNode, nodeTypeTunnelRelay, nodeTypeTunnelClient:
	default:
		return query, errors.New(errInvalidNodeListQuery)
	}
	return query, nil
}

// ListPage returns only the requested node page and latest apply metadata for its nodes.
func ListPage(ctx context.Context, input ListQuery) (*ListResult, error) {
	query, err := normalizeListQuery(input)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	nodes, total, err := repository.ListOpenFlareNodePage(ctx, model.OpenFlareNodeListQuery{
		Page: query.Page, PageSize: query.PageSize, Search: query.Search, Status: query.Status, NodeType: query.NodeType, Now: now,
	})
	if err != nil {
		return nil, err
	}
	views, err := buildNodeViews(ctx, nodes, now)
	if err != nil {
		return nil, err
	}
	return &ListResult{Total: total, Results: views, Page: query.Page, PageSize: query.PageSize}, nil
}

// GetNode returns a single node with the same latest apply metadata as the list.
func GetNode(ctx context.Context, id uint) (*View, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	views, err := buildNodeViews(ctx, []model.OpenFlareNode{*node}, time.Now())
	if err != nil {
		return nil, err
	}
	return views[0], nil
}
