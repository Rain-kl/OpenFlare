// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// ListTemplates returns all templates ordered by system flag and creation time.
func ListTemplates(ctx context.Context) ([]model.Template, error) {
	var templates []model.Template
	if err := db.DB(ctx).Order("is_system DESC, created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// GetTemplateByKey loads a template by its key.
func GetTemplateByKey(ctx context.Context, key string) (model.Template, error) {
	var tmpl model.Template
	if err := db.DB(ctx).Where("key = ?", key).First(&tmpl).Error; err != nil {
		return model.Template{}, err
	}
	return tmpl, nil
}

// TemplateExistsByKey reports whether a template key is already taken.
func TemplateExistsByKey(ctx context.Context, key string) (bool, error) {
	var existing model.Template
	err := db.DB(ctx).Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateTemplate persists a new template.
func CreateTemplate(ctx context.Context, tmpl *model.Template) error {
	return db.DB(ctx).Create(tmpl).Error
}

// SaveTemplate updates an existing template.
func SaveTemplate(ctx context.Context, tmpl *model.Template) error {
	return db.DB(ctx).Save(tmpl).Error
}

// DeleteTemplate removes a template record.
func DeleteTemplate(ctx context.Context, tmpl *model.Template) error {
	return db.DB(ctx).Delete(tmpl).Error
}
