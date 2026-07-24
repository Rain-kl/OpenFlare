// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/repository"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteIPGroupRejectsGraphReference(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	group, err := CreateIPGroup(ctx, IPGroupInput{Name: "trusted", Type: wafIPGroupTypeManual, Enabled: true})
	require.NoError(t, err)
	rule, err := CreateRule(ctx, CreateRuleInput{Name: "guard"})
	require.NoError(t, err)
	graph := RuleGraph{SchemaVersion: RuleGraphSchemaVersion, Nodes: []RuleNode{
		{ID: "start", Type: RuleNodeStart, Config: json.RawMessage(`{}`)},
		{ID: "match", Type: RuleNodeIPMatch, Config: json.RawMessage(`{"ip_group_ids":[` + strconv.FormatUint(uint64(group.ID), 10) + `]}`)},
		{ID: "allow", Type: RuleNodeAllow, Config: json.RawMessage(`{}`)},
	}, Edges: []RuleEdge{
		{ID: "e1", Source: "start", SourceHandle: "next", Target: "match"},
		{ID: "e2", Source: "match", SourceHandle: "true", Target: "allow"},
		{ID: "e3", Source: "match", SourceHandle: "false", Target: "allow"},
	}}
	_, err = SaveRuleGraph(ctx, rule.ID, SaveRuleGraphInput{Revision: rule.Revision, Graph: graph})
	require.NoError(t, err)

	view, err := GetIPGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, view.ReferencedByRuleCount)
	require.ErrorContains(t, DeleteIPGroup(ctx, group.ID), "已被 WAF 规则引用")
}

func TestRuleHandlersMapFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		setup  func(t *testing.T) func()
		want   int
	}{
		{name: "invalid id", method: http.MethodGet, path: "/rules/nope", setup: setupWAFTestDB, want: http.StatusBadRequest},
		{name: "malformed json", method: http.MethodPost, path: "/rules", body: `{`, setup: setupWAFTestDB, want: http.StatusBadRequest},
		{name: "invalid graph", method: http.MethodPost, path: "/rules/1/graph", body: `{"revision":1,"graph":{"schema_version":1,"nodes":[],"edges":[]}}`, setup: func(t *testing.T) func() {
			cleanup := setupWAFTestDB(t)
			_, err := CreateRule(context.Background(), CreateRuleInput{Name: "one"})
			require.NoError(t, err)
			return cleanup
		}, want: http.StatusBadRequest},
		{name: "manual IP group sync", method: http.MethodPost, path: "/ip-groups/1/sync", setup: func(t *testing.T) func() {
			cleanup := setupWAFTestDB(t)
			_, err := CreateIPGroup(context.Background(), IPGroupInput{Name: "manual", Type: wafIPGroupTypeManual, Enabled: true})
			require.NoError(t, err)
			return cleanup
		}, want: http.StatusBadRequest},
		{name: "missing", method: http.MethodGet, path: "/rules/999", setup: setupWAFTestDB, want: http.StatusNotFound},
		{name: "conflict", method: http.MethodPost, path: "/rules/1/graph", body: mustGraphRequest(t, 0), setup: func(t *testing.T) func() {
			cleanup := setupWAFTestDB(t)
			_, err := CreateRule(context.Background(), CreateRuleInput{Name: "one"})
			require.NoError(t, err)
			return cleanup
		}, want: http.StatusConflict},
		{name: "database failure", method: http.MethodGet, path: "/rules", setup: func(t *testing.T) func() { db.SetDB(nil); return func() {} }, want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup(t)
			defer cleanup()
			router := gin.New()
			router.Use(response.ErrorHandlerMiddleware())
			router.GET("/rules", ListRulesHandler)
			router.POST("/rules", CreateRuleHandler)
			router.GET("/rules/:id", GetRuleHandler)
			router.POST("/rules/:id/graph", SaveRuleGraphHandler)
			router.POST("/ip-groups/:id/sync", SyncIPGroupHandler)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			assert.Equal(t, tt.want, rec.Code, rec.Body.String())
		})
	}
}

func mustGraphRequest(t *testing.T, revision uint64) string {
	t.Helper()
	raw, err := json.Marshal(SaveRuleGraphInput{Revision: revision, Graph: DefaultRuleGraph()})
	require.NoError(t, err)
	return string(raw)
}

func TestCreateRuleCreatesDefaultGraph(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()

	rule, err := CreateRule(context.Background(), CreateRuleInput{Name: " edge guard "})
	require.NoError(t, err)
	assert.Equal(t, "edge guard", rule.Name)
	assert.False(t, rule.Enabled)
	assert.Equal(t, uint64(1), rule.Revision)
	assert.Equal(t, DefaultRuleGraph(), rule.Graph)
}

func TestCreateRuleRejectsEmptyName(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()

	_, err := CreateRule(context.Background(), CreateRuleInput{Name: "  "})
	require.ErrorContains(t, err, "名称不能为空")
}

func TestSaveRuleGraphValidationAndRevisionConflict(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	rule, err := CreateRule(ctx, CreateRuleInput{Name: "guard"})
	require.NoError(t, err)
	invalid := DefaultRuleGraph()
	invalid.Edges = nil
	_, err = SaveRuleGraph(ctx, rule.ID, SaveRuleGraphInput{Revision: rule.Revision, Graph: invalid})
	require.Error(t, err)

	updated, err := SaveRuleGraph(ctx, rule.ID, SaveRuleGraphInput{Revision: rule.Revision, Graph: DefaultRuleGraph()})
	require.NoError(t, err)
	assert.Equal(t, uint64(2), updated.Revision)
	_, err = SaveRuleGraph(ctx, rule.ID, SaveRuleGraphInput{Revision: rule.Revision, Graph: DefaultRuleGraph()})
	assert.ErrorIs(t, err, model.ErrWAFRuleRevisionConflict)
}

func TestReplaceSiteRuleGroupsPreservesOrderAndRejectsGlobal(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, db.DB(ctx).Create(&model.OriginProxyRoute{ID: 7, Domain: "example.com"}).Error)
	first, err := CreateRule(ctx, CreateRuleInput{Name: "first"})
	require.NoError(t, err)
	second, err := CreateRule(ctx, CreateRuleInput{Name: "second"})
	require.NoError(t, err)
	third, err := CreateRule(ctx, CreateRuleInput{Name: "third"})
	require.NoError(t, err)

	view, err := ReplaceSiteRuleGroups(ctx, 7, []uint{third.ID, first.ID, second.ID, first.ID})
	require.NoError(t, err)
	assert.Equal(t, []uint{third.ID, first.ID, second.ID}, view.AppliedIDs)

	require.NoError(t, EnsureDefaultRuleGroup(ctx))
	global, err := repository.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)
	_, err = ReplaceSiteRuleGroups(ctx, 7, []uint{global.ID, second.ID})
	require.Error(t, err)
	assert.False(t, errors.Is(err, model.ErrWAFRuleRevisionConflict))
	assert.Equal(t, []uint{third.ID, first.ID, second.ID}, mustListSiteRuleGroupIDs(t, ctx, 7))
}

func mustListSiteRuleGroupIDs(t *testing.T, ctx context.Context, routeID uint) []uint {
	t.Helper()
	ids, err := ListSiteRuleGroupIDs(ctx, routeID)
	require.NoError(t, err)
	return ids
}
