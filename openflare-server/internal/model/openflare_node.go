// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
)

// OpenFlareNode stores an edge, relay, or tunnel client node.
type OpenFlareNode struct {
	ID                        uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID                    string     `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Name                      string     `json:"name" gorm:"size:128;not null"`
	IP                        string     `json:"ip" gorm:"size:64;not null;default:''"`
	IPManualOverride          bool       `json:"ip_manual_override" gorm:"not null;default:false"`
	GeoName                   string     `json:"geo_name" gorm:"size:128;not null;default:''"`
	GeoLatitude               *float64   `json:"geo_latitude"`
	GeoLongitude              *float64   `json:"geo_longitude"`
	GeoManualOverride         bool       `json:"geo_manual_override" gorm:"not null;default:false"`
	AccessToken               string     `json:"-" gorm:"column:access_token;size:128;index"`
	AutoUpdateEnabled         bool       `json:"auto_update_enabled" gorm:"not null;default:false"`
	UpdateRequested           bool       `json:"update_requested" gorm:"not null;default:false"`
	UpdateChannel             string     `json:"update_channel" gorm:"size:16;not null;default:'stable'"`
	UpdateTag                 string     `json:"update_tag" gorm:"size:64;not null;default:''"`
	RestartOpenrestyRequested bool       `json:"restart_openresty_requested" gorm:"not null;default:false"`
	Version                   string     `json:"version" gorm:"size:64;not null;default:''"`
	ExtVersion                string     `json:"ext_version" gorm:"size:64;not null;default:''"`
	OpenrestyStatus           string     `json:"openresty_status" gorm:"size:16;not null;default:'unknown'"`
	OpenrestyMessage          string     `json:"openresty_message" gorm:"type:text"`
	Status                    string     `json:"status" gorm:"size:16;not null;default:'offline'"`
	CurrentVersion            string     `json:"current_version" gorm:"size:32;not null;default:''"`
	LastSeenAt                *time.Time `json:"last_seen_at"`
	LastError                 string     `json:"last_error" gorm:"type:text"`
	CreatedAt                 time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	NodeType                  string     `json:"node_type" gorm:"size:32;not null;default:'edge_node'"`
	RelayBindPort             int        `json:"relay_bind_port" gorm:"not null;default:0"`
	RelayVhostHTTPPort        int        `json:"relay_vhost_http_port" gorm:"not null;default:0"`
	RelayAuthToken            string     `json:"-" gorm:"size:128;not null;default:''"`
	RelayAgentAccessAddr      string     `json:"relay_agent_access_addr" gorm:"size:255;not null;default:''"`
	RelayClientAccessAddr     string     `json:"relay_client_access_addr" gorm:"size:255;not null;default:''"`
	RelayClientProxyURL       string     `json:"relay_client_proxy_url" gorm:"size:512;not null;default:''"`
	CapabilitiesJSON          string     `json:"capabilities_json" gorm:"type:text;not null;default:'[]'"`
	RelayStatus               string     `json:"relay_status" gorm:"size:16;not null;default:'unknown'"`
	RelayWebServerEnabled     bool       `json:"relay_web_server_enabled" gorm:"not null;default:false"`
}

// TableName returns the GORM table name.
func (OpenFlareNode) TableName() string {
	return "of_nodes"
}

// ListOpenFlareNodes returns all nodes ordered by id desc.
func ListOpenFlareNodes(ctx context.Context) ([]OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var nodes []OpenFlareNode
	if err := conn.Order("id desc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// ListOpenFlareNodesByNodeIDs returns nodes matching the given node ids.
func ListOpenFlareNodesByNodeIDs(ctx context.Context, nodeIDs []string) ([]OpenFlareNode, error) {
	if len(nodeIDs) == 0 {
		return []OpenFlareNode{}, nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var nodes []OpenFlareNode
	if err := conn.Where("node_id IN ?", nodeIDs).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetOpenFlareNodeByID returns a node by primary key.
func GetOpenFlareNodeByID(ctx context.Context, id uint) (*OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var node OpenFlareNode
	if err := conn.First(&node, id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// GetOpenFlareNodeByNodeID returns a node by node_id.
func GetOpenFlareNodeByNodeID(ctx context.Context, nodeID string) (*OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var node OpenFlareNode
	if err := conn.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// GetOpenFlareNodeByAccessToken returns a node by access token.
func GetOpenFlareNodeByAccessToken(ctx context.Context, token string) (*OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var node OpenFlareNode
	if err := conn.Where("access_token = ?", token).First(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// CreateOpenFlareNode inserts a new node.
func CreateOpenFlareNode(ctx context.Context, node *OpenFlareNode) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(node).Error
}

// SaveOpenFlareNode persists node changes.
func SaveOpenFlareNode(ctx context.Context, node *OpenFlareNode) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Save(node).Error
}

// UpdateOpenFlareNodeFields updates selected columns for a node.
func UpdateOpenFlareNodeFields(ctx context.Context, node *OpenFlareNode, fields ...string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	if len(fields) == 0 {
		return conn.Save(node).Error
	}
	return conn.Model(node).Select(fields).Updates(node).Error
}

// DeleteOpenFlareNode removes a node by primary key.
func DeleteOpenFlareNode(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&OpenFlareNode{}, id).Error
}
