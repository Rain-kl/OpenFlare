package model

import "time"

type NodeHealthEvent struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	NodeID           string     `json:"node_id" gorm:"index;size:64;not null"`
	EventType        string     `json:"event_type" gorm:"index;size:64;not null"`
	Severity         string     `json:"severity" gorm:"size:16;not null"`
	Status           string     `json:"status" gorm:"index;size:16;not null"`
	Message          string     `json:"message" gorm:"type:text"`
	FirstTriggeredAt time.Time  `json:"first_triggered_at" gorm:"index"`
	LastTriggeredAt  time.Time  `json:"last_triggered_at" gorm:"index"`
	ReportedAt       time.Time  `json:"reported_at" gorm:"index"`
	ResolvedAt       *time.Time `json:"resolved_at" gorm:"index"`
	RawJSON          string     `json:"raw_json" gorm:"type:text"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func GetActiveNodeHealthEvent(nodeID string, eventType string) (*NodeHealthEvent, error) {
	event := &NodeHealthEvent{}
	err := DB.Where("node_id = ? AND event_type = ? AND status = ?", nodeID, eventType, "active").First(event).Error
	return event, err
}

func ListNodeHealthEvents(nodeID string, activeOnly bool, limit int) (events []*NodeHealthEvent, err error) {
	query := DB.Where("node_id = ?", nodeID).Order("last_triggered_at desc")
	if activeOnly {
		query = query.Where("status = ?", "active")
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err = query.Find(&events).Error
	return events, err
}

func ListActiveNodeHealthEvents() (events []*NodeHealthEvent, err error) {
	err = DB.Where("status = ?", "active").Order("last_triggered_at desc").Find(&events).Error
	return events, err
}

func DeleteNodeHealthEvents(nodeID string) (deleted int64, err error) {
	result := DB.Where("node_id = ?", nodeID).Delete(&NodeHealthEvent{})
	return result.RowsAffected, result.Error
}
