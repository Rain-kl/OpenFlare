// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPagesSourceModelsMatchMigrationSchema(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(
		&PagesProject{},
		&PagesDeployment{},
		&PagesProjectSource{},
		&PagesProjectSourceRuntime{},
	))

	assert.Equal(t, "of_pages_project_sources", (PagesProjectSource{}).TableName())
	assert.Equal(t, "of_pages_project_source_runtime", (PagesProjectSourceRuntime{}).TableName())
	assert.True(t, gormDB.Migrator().HasColumn(&PagesProjectSource{}, "github_repository"))
	assert.False(t, gormDB.Migrator().HasColumn(&PagesProjectSource{}, "git_hub_repository"))
	assert.True(t, gormDB.Migrator().HasColumn(&PagesProjectSourceRuntime{}, "etag"))
	assert.False(t, gormDB.Migrator().HasColumn(&PagesProjectSourceRuntime{}, "e_tag"))

	var indexSQL string
	require.NoError(t, gormDB.Raw(
		"SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?",
		"idx_of_pages_deployments_source_revision",
	).Scan(&indexSQL).Error)
	assert.Contains(t, strings.ToUpper(indexSQL), "WHERE SOURCE_IDENTITY IS NOT NULL AND SOURCE_REVISION IS NOT NULL")
}

func TestPagesSourceModelsDoNotSerializeSecretsOrFencingState(t *testing.T) {
	sourceJSON, err := json.Marshal(PagesProjectSource{
		ID:             1,
		ProjectID:      2,
		RemoteURL:      "https://example.com/site.zip?token=secret",
		ConfigVersion:  3,
		SourceIdentity: strings.Repeat("a", 64),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(sourceJSON))

	runtimeJSON, err := json.Marshal(PagesProjectSourceRuntime{
		SourceID:   1,
		ETag:       `"secret-etag"`,
		LeaseToken: "secret-lease",
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(runtimeJSON))

	identity := strings.Repeat("b", 64)
	revision := strings.Repeat("c", 64)
	deploymentJSON, err := json.Marshal(PagesDeployment{
		SourceType:     "remote_url",
		SourceIdentity: &identity,
		SourceRevision: &revision,
		SourceLabel:    "site.zip",
		SourceMeta:     `{"provider":"remote_url","private":"secret"}`,
		TriggerType:    "manual_sync",
	})
	require.NoError(t, err)
	assert.NotContains(t, string(deploymentJSON), identity)
	assert.NotContains(t, string(deploymentJSON), revision)
	assert.NotContains(t, string(deploymentJSON), "private")
	assert.Contains(t, string(deploymentJSON), `"source_type":"remote_url"`)
	assert.Contains(t, string(deploymentJSON), `"source_label":"site.zip"`)
	assert.Contains(t, string(deploymentJSON), `"trigger_type":"manual_sync"`)
}
