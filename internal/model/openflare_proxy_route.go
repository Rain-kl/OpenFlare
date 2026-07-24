// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// ProxyRoute OpenFlare 代理规则实体。
// 域名与证书仅通过 of_zone_domains 关联，不再持久化在本表。
type ProxyRoute struct {
	ID                   uint         `json:"id" gorm:"primaryKey;autoIncrement"`
	SiteName             string       `json:"site_name" gorm:"size:255;not null;default:''"`
	OriginID             *uint        `json:"origin_id" gorm:"index"`
	OriginURL            string       `json:"origin_url" gorm:"size:2048;not null"`
	OriginHost           string       `json:"origin_host" gorm:"size:255"`
	Upstreams            string       `json:"upstreams" gorm:"type:text;not null;default:'[]'"`
	Enabled              bool         `json:"enabled" gorm:"not null;default:true"`
	EnableHTTPS          bool         `json:"enable_https" gorm:"column:enable_https;not null;default:false"`
	RedirectHTTP         bool         `json:"redirect_http" gorm:"not null;default:false"`
	LimitConnPerServer   int          `json:"limit_conn_per_server" gorm:"not null;default:0"`
	LimitConnPerIP       int          `json:"limit_conn_per_ip" gorm:"not null;default:0"`
	LimitRate            string       `json:"limit_rate" gorm:"size:32;not null;default:''"`
	LimitReqPerIP        string       `json:"limit_req_per_ip" gorm:"size:32;not null;default:''"`
	CacheEnabled         bool         `json:"cache_enabled" gorm:"not null;default:false"`
	CachePolicy          string       `json:"cache_policy" gorm:"size:32;not null;default:''"`
	CacheRules           string       `json:"cache_rules" gorm:"type:text;not null;default:'[]'"`
	CustomHeaders        string       `json:"custom_headers" gorm:"type:text;not null;default:'[]'"`
	BasicAuthEnabled     bool         `json:"basic_auth_enabled" gorm:"not null;default:false"`
	BasicAuthUsername    string       `json:"basic_auth_username" gorm:"size:255;not null;default:''"`
	BasicAuthPassword    string       `json:"basic_auth_password" gorm:"size:255;not null;default:''"`
	UpstreamType         string       `json:"upstream_type" gorm:"size:32;not null;default:'direct'"`
	TunnelNodeID         *uint        `json:"tunnel_node_id" gorm:"index"`
	TunnelTargetAddr     string       `json:"tunnel_target_addr" gorm:"size:512"`
	TunnelTargetProtocol string       `json:"tunnel_target_protocol" gorm:"size:16"`
	PagesProjectID       *uint        `json:"pages_project_id" gorm:"index"`
	CreatedAt            time.Time    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time    `json:"updated_at" gorm:"autoUpdateTime"`
	ZoneDomains          []ZoneDomain `json:"zone_domains,omitempty" gorm:"foreignKey:ProxyRouteID"`
}

// TableName 表名。
func (ProxyRoute) TableName() string {
	return tableOfProxyRoutes
}
