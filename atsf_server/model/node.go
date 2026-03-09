package model

import "time"

type Node struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	NodeID         string    `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Name           string    `json:"name" gorm:"size:128;not null"`
	IP             string    `json:"ip" gorm:"size:64;not null"`
	AgentVersion   string    `json:"agent_version" gorm:"size:64;not null"`
	NginxVersion   string    `json:"nginx_version" gorm:"size:64"`
	Status         string    `json:"status" gorm:"size:16;not null;default:'offline'"`
	CurrentVersion string    `json:"current_version" gorm:"size:32"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	LastError      string    `json:"last_error" gorm:"size:1024"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func ListNodes() (nodes []*Node, err error) {
	err = DB.Order("id desc").Find(&nodes).Error
	return nodes, err
}

func GetNodeByNodeID(nodeID string) (*Node, error) {
	node := &Node{}
	err := DB.Where("node_id = ?", nodeID).First(node).Error
	return node, err
}
