// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"bytes"
	"errors"
	"strings"
	"text/template"
	"time"
)

// Template 邮件/消息模板实体
type Template struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"uniqueIndex;size:80;not null"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Type        string    `json:"type" gorm:"size:20;not null;default:'email'"`
	Subject     string    `json:"subject" gorm:"size:255"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"size:255"`
	IsSystem    bool      `json:"is_system" gorm:"index;not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 表名
func (Template) TableName() string {
	return "w_templates"
}

// Normalize 规范化模板字段
func (t *Template) Normalize() {
	t.Key = strings.TrimSpace(t.Key)
	t.Name = strings.TrimSpace(t.Name)
	t.Type = strings.ToLower(strings.TrimSpace(t.Type))
	t.Subject = strings.TrimSpace(t.Subject)
	t.Content = strings.TrimSpace(t.Content)
	t.Description = strings.TrimSpace(t.Description)
	if t.Type == "" {
		t.Type = "email"
	}
}

// Validate 校验模板必填字段
func (t *Template) Validate() error {
	t.Normalize()
	if t.Key == "" {
		return errors.New(errTemplateKeyRequired)
	}
	if t.Name == "" {
		return errors.New(errTemplateNameRequired)
	}
	if t.Content == "" {
		return errors.New(errTemplateContentRequired)
	}
	return nil
}

// Render 渲染模板的 Subject 和 Content
func (t *Template) Render(data any) (string, string, error) {
	// Render Subject
	var subject string
	if t.Subject != "" {
		tmplSubject, err := template.New(t.Key + "_subject").Parse(t.Subject)
		if err != nil {
			return "", "", err
		}
		var subBuf bytes.Buffer
		if err := tmplSubject.Execute(&subBuf, data); err != nil {
			return "", "", err
		}
		subject = subBuf.String()
	}

	// Render Content
	tmplContent, err := template.New(t.Key + "_content").Parse(t.Content)
	if err != nil {
		return "", "", err
	}
	var bodyBuf bytes.Buffer
	if err := tmplContent.Execute(&bodyBuf, data); err != nil {
		return "", "", err
	}

	return subject, bodyBuf.String(), nil
}
