package model

import "time"

type Node struct {
	ID                        uint      `json:"id" gorm:"primaryKey"`
	NodeID                    string    `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Name                      string    `json:"name" gorm:"size:128;not null"`
	IP                        string    `json:"ip" gorm:"size:64;not null"`
	IPManualOverride          bool      `json:"ip_manual_override" gorm:"not null;default:false"`
	GeoName                   string    `json:"geo_name" gorm:"size:128"`
	GeoLatitude               *float64  `json:"geo_latitude"`
	GeoLongitude              *float64  `json:"geo_longitude"`
	GeoManualOverride         bool      `json:"geo_manual_override" gorm:"not null;default:false"`
	AccessToken               string    `json:"-" gorm:"column:access_token;size:128;index"`
	AutoUpdateEnabled         bool      `json:"auto_update_enabled" gorm:"not null;default:false"`
	UpdateRequested           bool      `json:"update_requested" gorm:"not null;default:false"`
	UpdateChannel             string    `json:"update_channel" gorm:"size:16;not null;default:'stable'"`
	UpdateTag                 string    `json:"update_tag" gorm:"size:64"`
	RestartOpenrestyRequested bool      `json:"restart_openresty_requested" gorm:"not null;default:false"`
	Version                   string    `json:"version" gorm:"size:64;not null;default:''"`
	ExtVersion                string    `json:"ext_version" gorm:"size:64"`
	OpenrestyStatus           string    `json:"openresty_status" gorm:"size:16;not null;default:'unknown'"`
	OpenrestyMessage          string    `json:"openresty_message" gorm:"type:text"`
	Status                    string    `json:"status" gorm:"size:16;not null;default:'offline'"`
	CurrentVersion            string    `json:"current_version" gorm:"size:32"`
	LastSeenAt                time.Time `json:"last_seen_at"`
	LastError                 string    `json:"last_error" gorm:"type:text"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
	// Node type: edge_node (default) | tunnel_relay | tunnel_client
	NodeType string `json:"node_type" gorm:"size:32;not null;default:'edge_node'"`
	// TunnelRelay specific fields
	RelayBindPort         int    `json:"relay_bind_port" gorm:"not null;default:0"`
	RelayVhostHTTPPort    int    `json:"relay_vhost_http_port" gorm:"not null;default:0"`
	RelayAuthToken        string `json:"-" gorm:"size:128"`
	RelayAgentAccessAddr  string `json:"relay_agent_access_addr" gorm:"size:255"`
	RelayClientAccessAddr string `json:"relay_client_access_addr" gorm:"size:255"`
	RelayClientProxyURL   string `json:"relay_client_proxy_url" gorm:"size:512"`
	CapabilitiesJSON      string `json:"capabilities_json" gorm:"type:text;not null;default:'[]'"`
	RelayStatus           string `json:"relay_status" gorm:"size:16;not null;default:'unknown'"`
	RelayWebServerEnabled bool   `json:"relay_web_server_enabled" gorm:"not null;default:false"`
}

func ListNodes() (nodes []*Node, err error) {
	err = DB.Order("id desc").Find(&nodes).Error
	return nodes, err
}

func ListNodesByNodeIDs(nodeIDs []string) (nodes []*Node, err error) {
	if len(nodeIDs) == 0 {
		return []*Node{}, nil
	}
	err = DB.Where("node_id IN ?", nodeIDs).Find(&nodes).Error
	return nodes, err
}

func GetNodeByNodeID(nodeID string) (*Node, error) {
	node := &Node{}
	err := DB.Where("node_id = ?", nodeID).First(node).Error
	return node, err
}

func GetNodeByID(id uint) (*Node, error) {
	node := &Node{}
	err := DB.First(node, id).Error
	return node, err
}

func GetNodeByAccessToken(token string) (*Node, error) {
	node := &Node{}
	err := DB.Where("access_token = ?", token).First(node).Error
	return node, err
}

func (node *Node) Insert() error {
	return DB.Create(node).Error
}

func (node *Node) Update() error {
	return DB.Save(node).Error
}

func (node *Node) Delete() error {
	return DB.Delete(node).Error
}

func ListNodesByType(nodeType string) (nodes []*Node, err error) {
	err = DB.Where("node_type = ?", nodeType).Order("id desc").Find(&nodes).Error
	return nodes, err
}
