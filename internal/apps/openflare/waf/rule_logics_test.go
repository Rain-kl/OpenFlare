// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"bytes"
	"context"
	"encoding/json"
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
	require.ErrorContains(t, DeleteIPGroup(ctx, group.ID), "referenced by WAF rules")
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
			t.Helper()
			cleanup := setupWAFTestDB(t)
			_, err := CreateRule(context.Background(), CreateRuleInput{Name: "one"})
			require.NoError(t, err)
			return cleanup
		}, want: http.StatusBadRequest},
		{name: "manual IP group sync", method: http.MethodPost, path: "/ip-groups/1/sync", setup: func(t *testing.T) func() {
			t.Helper()
			cleanup := setupWAFTestDB(t)
			_, err := CreateIPGroup(context.Background(), IPGroupInput{Name: "manual", Type: wafIPGroupTypeManual, Enabled: true})
			require.NoError(t, err)
			return cleanup
		}, want: http.StatusBadRequest},
		{name: "missing", method: http.MethodGet, path: "/rules/999", setup: setupWAFTestDB, want: http.StatusNotFound},
		{name: "conflict", method: http.MethodPost, path: "/rules/1/graph", body: mustGraphRequest(t, 0), setup: func(t *testing.T) func() {
			t.Helper()
			cleanup := setupWAFTestDB(t)
			_, err := CreateRule(context.Background(), CreateRuleInput{Name: "one"})
			require.NoError(t, err)
			return cleanup
		}, want: http.StatusConflict},
		{name: "database failure", method: http.MethodGet, path: "/rules", setup: func(t *testing.T) func() { t.Helper(); db.SetDB(nil); return func() {} }, want: http.StatusInternalServerError},
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

func TestRuleGroupSitesHandlersListAndRemoveSelectedBindings(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()
	conn := db.DB(ctx)
	require.NoError(t, conn.AutoMigrate(&model.ProxyRoute{}, &model.ZoneDomain{}))
	routes := []model.ProxyRoute{
		{SiteName: "Alpha", OriginURL: "https://origin.example"},
		{SiteName: "Beta", OriginURL: "https://origin.example"},
		{SiteName: "Gamma", OriginURL: "https://origin.example"},
	}
	require.NoError(t, conn.Create(&routes).Error)
	require.NoError(t, conn.Create(&model.ZoneDomain{ZoneID: 1, ProxyRouteID: &routes[0].ID, Domain: "alpha.example"}).Error)

	group, err := CreateRule(ctx, CreateRuleInput{Name: "guard"})
	require.NoError(t, err)
	other, err := CreateRule(ctx, CreateRuleInput{Name: "other"})
	require.NoError(t, err)
	last, err := CreateRule(ctx, CreateRuleInput{Name: "last"})
	require.NoError(t, err)
	require.NoError(t, repository.ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx, routes[0].ID, []uint{other.ID, group.ID, last.ID}))
	require.NoError(t, repository.ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx, routes[1].ID, []uint{group.ID}))
	require.NoError(t, repository.ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx, routes[2].ID, []uint{group.ID}))
	require.NoError(t, conn.Create(&model.OpenFlareWAFRuleGroupBinding{RuleGroupID: group.ID, ProxyRouteID: 999}).Error)
	require.NoError(t, EnsureDefaultRuleGroup(ctx))
	global, err := repository.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)

	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	router.GET("/rules/:id/sites", ListRuleGroupSitesHandler)
	router.POST("/rules/:id/sites/remove", RemoveRuleGroupSitesHandler)
	request := func(method string, id uint, suffix string, body []byte, status int) *httptest.ResponseRecorder {
		t.Helper()
		url := "/rules/" + strconv.FormatUint(uint64(id), 10) + "/sites" + suffix
		req := httptest.NewRequest(method, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, status, rec.Code, rec.Body.String())
		return rec
	}
	list := func(id uint) []RuleGroupSite {
		t.Helper()
		var result struct {
			Data []RuleGroupSite `json:"data"`
		}
		rec := request(http.MethodGet, id, "", nil, http.StatusOK)
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		return result.Data
	}
	assert.Equal(t, []RuleGroupSite{
		{ID: routes[0].ID, SiteName: "Alpha", Domains: []string{"alpha.example"}},
		{ID: routes[1].ID, SiteName: "Beta", Domains: []string{}},
		{ID: routes[2].ID, SiteName: "Gamma", Domains: []string{}},
	}, list(group.ID))

	payload, err := json.Marshal(IDsRequest{IDs: []uint{routes[0].ID, routes[1].ID, routes[0].ID}})
	require.NoError(t, err)
	request(http.MethodPost, global.ID, "/remove", payload, http.StatusBadRequest)
	request(http.MethodPost, group.ID, "/remove", []byte(`{"ids":[0]}`), http.StatusBadRequest)
	result := request(http.MethodPost, group.ID, "/remove", payload, http.StatusOK)
	assert.JSONEq(t, `{"error_msg":"","data":{"removed_count":2}}`, result.Body.String())
	assert.Equal(t, []RuleGroupSite{{ID: routes[2].ID, SiteName: "Gamma", Domains: []string{}}}, list(group.ID))
	assert.Len(t, list(global.ID), 3)

	view, err := GetRule(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{routes[2].ID}, view.AppliedSiteIDs)
	ids, err := ListSiteRuleGroupIDs(ctx, routes[0].ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{other.ID, last.ID}, ids)
	retry := request(http.MethodPost, group.ID, "/remove", payload, http.StatusOK)
	assert.JSONEq(t, `{"error_msg":"","data":{"removed_count":0}}`, retry.Body.String())
}

func TestCreateRuleRejectsEmptyName(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()

	_, err := CreateRule(context.Background(), CreateRuleInput{Name: "  "})
	require.ErrorContains(t, err, "name must not be empty")
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
	require.NotErrorIs(t, err, model.ErrWAFRuleRevisionConflict)
	assert.Equal(t, []uint{third.ID, first.ID, second.ID}, mustListSiteRuleGroupIDs(t, ctx, 7))
}

func mustListSiteRuleGroupIDs(t *testing.T, ctx context.Context, routeID uint) []uint {
	t.Helper()
	ids, err := ListSiteRuleGroupIDs(ctx, routeID)
	require.NoError(t, err)
	return ids
}
