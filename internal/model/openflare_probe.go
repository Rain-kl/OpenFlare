// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringSliceJSON persists a slice of strings as a JSON text array (mirrors Komari StringArray).
type StringSliceJSON []string

// Scan decodes JSON text/bytes into a string slice.
func (s *StringSliceJSON) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*s = StringSliceJSON{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported StringSliceJSON scan type: %T", value)
	}
	if len(raw) == 0 {
		*s = StringSliceJSON{}
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if out == nil {
		out = []string{}
	}
	*s = StringSliceJSON(out)
	return nil
}

// Value encodes the string slice as JSON text.
func (s StringSliceJSON) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	raw, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// OpenFlarePingTask defines a scheduled latency probe (mirrors Komari PingTask).
type OpenFlarePingTask struct {
	ID        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Weight    int             `json:"weight" gorm:"column:weight;not null;default:0;index"`
	Name      string          `json:"name" gorm:"column:name;size:255;not null;index"`
	Clients   StringSliceJSON `json:"clients" gorm:"column:clients;type:text;not null;default:'[]'"`
	DefaultOn bool            `json:"default_on" gorm:"column:all_clients;not null;default:false"`
	Type      string          `json:"type" gorm:"column:type;size:16;not null;default:'icmp'"`
	Target    string          `json:"target" gorm:"column:target;size:255;not null"`
	Interval  int             `json:"interval" gorm:"column:interval;not null;default:60"`
	CreatedAt time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlarePingTask) TableName() string {
	return "of_ping_tasks"
}

// OpenFlarePingRecord stores a single latency probe measurement in logstore (mirrors Komari PingRecord).
type OpenFlarePingRecord struct {
	ID         uint64    `json:"id,omitempty" gorm:"column:id;primaryKey"`
	NodeID     string    `json:"node_id" gorm:"column:node_id;index;size:64;not null"`
	TaskID     uint      `json:"task_id" gorm:"column:task_id;index;not null"`
	RecordedAt time.Time `json:"recorded_at" gorm:"column:recorded_at;index;not null"`
	Value      int       `json:"value" gorm:"column:value;not null;default:-1"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the table name.
func (OpenFlarePingRecord) TableName() string {
	return "of_node_ping_records"
}

// OpenFlareExecTask stores a remote command execution dispatch (mirrors Komari Task).
type OpenFlareExecTask struct {
	TaskID    string                    `json:"task_id" gorm:"column:task_id;size:64;primaryKey"`
	Clients   StringSliceJSON           `json:"clients" gorm:"column:clients;type:text;not null;default:'[]'"`
	Command   string                    `json:"command" gorm:"column:command;type:text;not null"`
	CreatedBy uint                      `json:"created_by" gorm:"column:created_by;not null;default:0"`
	CreatedAt time.Time                 `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	Results   []OpenFlareExecTaskResult `json:"results,omitempty" gorm:"-"`
}

// TableName returns the GORM table name.
func (OpenFlareExecTask) TableName() string {
	return "of_exec_tasks"
}

// OpenFlareExecTaskResult stores per-node remote command execution output (mirrors Komari TaskResult).
type OpenFlareExecTaskResult struct {
	ID         uint       `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	TaskID     string     `json:"task_id" gorm:"column:task_id;size:64;not null;index"`
	Client     string     `json:"client" gorm:"column:client;size:64;not null;index"`
	Result     string     `json:"result" gorm:"column:result;type:text;not null;default:''"`
	ExitCode   *int       `json:"exit_code" gorm:"column:exit_code"`
	FinishedAt *time.Time `json:"finished_at" gorm:"column:finished_at"`
	CreatedAt  time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareExecTaskResult) TableName() string {
	return "of_exec_task_results"
}
