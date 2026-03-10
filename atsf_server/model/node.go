package model

import "time"

type Node struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	NodeID         string    `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Name           string    `json:"name" gorm:"size:128;not null"`
	IP             string    `json:"ip" gorm:"size:64;not null"`
	AgentToken     string    `json:"-" gorm:"size:128;uniqueIndex"`
	DiscoveryToken string    `json:"-" gorm:"size:128;uniqueIndex"`
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

func GetNodeByDiscoveryToken(token string) (*Node, error) {
	node := &Node{}
	err := DB.Where("discovery_token = ?", token).First(node).Error
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
