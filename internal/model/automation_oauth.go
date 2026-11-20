package model

import "time"

// AutomationCode stores a single-use authorization code by hash.
type AutomationCode struct {
	Hash        string `gorm:"primaryKey"`
	UserID      uint64
	ClientID    string
	RedirectURI string
	Resource    string
	Scope       string
	Challenge   string
	ExpiresAt   time.Time
	Used        bool
}

// TableName returns the authorization code table.
func (AutomationCode) TableName() string { return "w_automation_codes" }

// AutomationGrant stores a revocable OAuth grant and hashed credentials.
type AutomationGrant struct {
	ID               string `gorm:"primaryKey"`
	UserID           uint64
	ClientID         string
	Resource         string
	Scope            string
	AccessHash       string
	RefreshHash      string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// TableName returns the OAuth grants table.
func (AutomationGrant) TableName() string { return "w_automation_grants" }
