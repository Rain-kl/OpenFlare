// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebindSnapshotPagesToCurrentActive(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:    "Rebind Site",
		Slug:    "rebind-site",
		Enabled: true,
	})
	require.NoError(t, err)

	old := &model.PagesDeployment{
		ProjectID:        project.ID,
		DeploymentNumber: 1,
		Checksum:         "old-checksum",
		Status:           model.PagesDeploymentStatusUploaded,
		FileCount:        1,
	}
	require.NoError(t, db.DB(ctx).Create(old).Error)
	active := &model.PagesDeployment{
		ProjectID:        project.ID,
		DeploymentNumber: 2,
		Checksum:         "new-checksum",
		Status:           model.PagesDeploymentStatusActive,
		FileCount:        1,
	}
	require.NoError(t, db.DB(ctx).Create(active).Error)
	require.NoError(t, db.DB(ctx).Model(&model.PagesProject{}).
		Where("id = ?", project.ID).
		Update("active_deployment_id", active.ID).Error)

	// Frozen snapshot still points at the old deployment (simulates old main config).
	frozen := map[string]any{
		"routes": []map[string]any{
			{
				"site_name":        "rebind",
				"origin_url":       "openflare-pages://project/1",
				"enabled":          true,
				"upstream_type":    "pages",
				"pages_project_id": project.ID,
				"pages_deployment": map[string]any{
					"project_id":    project.ID,
					"deployment_id": old.ID,
					"checksum":      "old-checksum",
					"local_root":    "__OPENFLARE_PAGES_DIR__/deployments/1/current",
				},
				"extra_keep_me": "yes",
			},
		},
		"waf": map[string]any{"rule_groups": []any{}},
	}
	frozenJSON, err := json.Marshal(frozen)
	require.NoError(t, err)

	reboundJSON, err := RebindSnapshotPagesToCurrentActive(ctx, string(frozenJSON))
	require.NoError(t, err)

	var rebound map[string]any
	require.NoError(t, json.Unmarshal([]byte(reboundJSON), &rebound))
	_, hasWAF := rebound["waf"]
	assert.True(t, hasWAF)

	routes := rebound["routes"].([]any)
	require.Len(t, routes, 1)
	route := routes[0].(map[string]any)
	assert.Equal(t, "yes", route["extra_keep_me"])
	deployment := route["pages_deployment"].(map[string]any)
	assert.EqualValues(t, active.ID, deployment["deployment_id"])
	assert.Equal(t, "new-checksum", deployment["checksum"])
}
