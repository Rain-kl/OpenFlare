package model

import "time"

type Node struct {
	ID                        uint      `json:"id" gorm:"primaryKey"`
	NodeID                    string    `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Name                      string    `json:"name" gorm:"size:128;not null"`
	IP                        string    `json:"ip" gorm:"size:64;not null"`
	GeoName                   string    `json:"geo_name" gorm:"size:128"`
	GeoLatitude               *float64  `json:"geo_latitude"`
	GeoLongitude              *float64  `json:"geo_longitude"`
	GeoManualOverride         bool      `json:"geo_manual_override" gorm:"not null;default:false"`
	AgentToken                string    `json:"-" gorm:"size:128;index"`
	AutoUpdateEnabled         bool      `json:"auto_update_enabled" gorm:"not null;default:false"`
	UpdateRequested           bool      `json:"update_requested" gorm:"not null;default:false"`
	UpdateChannel             string    `json:"update_channel" gorm:"size:16;not null;default:'stable'"`
	UpdateTag                 string    `json:"update_tag" gorm:"size:64"`
	RestartOpenrestyRequested bool      `json:"restart_openresty_requested" gorm:"not null;default:false"`
	AgentVersion              string    `json:"agent_version" gorm:"size:64;not null"`
	NginxVersion              string    `json:"nginx_version" gorm:"size:64"`
	OpenrestyStatus           string    `json:"openresty_status" gorm:"size:16;not null;default:'unknown'"`
	OpenrestyMessage          string    `json:"openresty_message" gorm:"size:2048"`
	Status                    string    `json:"status" gorm:"size:16;not null;default:'offline'"`
	CurrentVersion            string    `json:"current_version" gorm:"size:32"`
	LastSeenAt                time.Time `json:"last_seen_at"`
	LastError                 string    `json:"last_error" gorm:"size:1024"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
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

func GetNodeByAgentToken(token string) (*Node, error) {
	node := &Node{}
	err := DB.Where("agent_token = ?", token).First(node).Error
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
