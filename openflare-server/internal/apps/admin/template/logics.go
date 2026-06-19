// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func createTemplate(ctx context.Context, req CreateTemplateRequest) (model.Template, error) {
	exists, err := repository.TemplateExistsByKey(ctx, req.Key)
	if err != nil {
		return model.Template{}, err
	}
	if exists {
		return model.Template{}, errors.New(TemplateKeyExists)
	}

	tmpl := model.Template{
		Key:         req.Key,
		Name:        req.Name,
		Type:        req.Type,
		Subject:     req.Subject,
		Content:     req.Content,
		Description: req.Description,
		IsSystem:    false,
	}
	if err := tmpl.Validate(); err != nil {
		return model.Template{}, err
	}
	if err := repository.CreateTemplate(ctx, &tmpl); err != nil {
		return model.Template{}, err
	}
	return tmpl, nil
}

func listTemplates(ctx context.Context) ([]model.Template, error) {
	return repository.ListTemplates(ctx)
}

func getTemplate(ctx context.Context, key string) (model.Template, error) {
	return repository.GetTemplateByKey(ctx, key)
}

func updateTemplate(ctx context.Context, key string, req UpdateTemplateRequest) (model.Template, error) {
	tmpl, err := repository.GetTemplateByKey(ctx, key)
	if err != nil {
		return model.Template{}, err
	}

	tmpl.Name = req.Name
	tmpl.Type = req.Type
	tmpl.Subject = req.Subject
	tmpl.Content = req.Content
	tmpl.Description = req.Description
	if err := tmpl.Validate(); err != nil {
		return model.Template{}, err
	}
	if err := repository.SaveTemplate(ctx, &tmpl); err != nil {
		return model.Template{}, err
	}
	return tmpl, nil
}

func deleteTemplate(ctx context.Context, key string) error {
	tmpl, err := repository.GetTemplateByKey(ctx, key)
	if err != nil {
		return err
	}
	if tmpl.IsSystem {
		return errors.New(SystemTemplateCannotDelete)
	}
	return repository.DeleteTemplate(ctx, &tmpl)
}
