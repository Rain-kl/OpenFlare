// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestGroupFromInputTTL(t *testing.T) {
	auto, manual, minimum, maximum := 1, 120, 60, 86400
	tests := []struct {
		name     string
		existing *model.CFPointingGroup
		ttl      *int
		want     int
	}{
		{name: "new_default_auto", want: 1},
		{name: "explicit_auto", ttl: &auto, want: 1},
		{name: "manual", ttl: &manual, want: 120},
		{name: "minimum", ttl: &minimum, want: 60},
		{name: "maximum", ttl: &maximum, want: 86400},
		{name: "legacy_default", existing: &model.CFPointingGroup{}, want: 300},
		{name: "preserve_manual", existing: &model.CFPointingGroup{TTL: 600}, want: 600},
		{name: "preserve_auto", existing: &model.CFPointingGroup{TTL: 1}, want: 1},
		{name: "manual_to_auto", existing: &model.CFPointingGroup{TTL: 600}, ttl: &auto, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, err := groupFromInput(context.Background(), tt.existing, GroupInput{
				Name: "custom", TargetMode: "custom", RecordType: "CNAME", RecordContent: "origin.example.com", TTL: tt.ttl,
			})
			if err != nil {
				t.Fatalf("groupFromInput(%s) error = %v", tt.name, err)
			}
			if group.TTL != tt.want {
				t.Errorf("groupFromInput(%s).TTL = %d, want %d", tt.name, group.TTL, tt.want)
			}
		})
	}
}

func TestCreateGroupHandlerTTL(t *testing.T) {
	tests := []struct {
		name       string
		ttlJSON    string
		wantStatus int
		wantTTL    int
	}{
		{name: "omitted", wantStatus: http.StatusOK, wantTTL: 1},
		{name: "auto", ttlJSON: `,"ttl":1`, wantStatus: http.StatusOK, wantTTL: 1},
		{name: "manual", ttlJSON: `,"ttl":120`, wantStatus: http.StatusOK, wantTTL: 120},
		{name: "zero", ttlJSON: `,"ttl":0`, wantStatus: http.StatusBadRequest},
		{name: "negative", ttlJSON: `,"ttl":-1`, wantStatus: http.StatusBadRequest},
		{name: "below_minimum", ttlJSON: `,"ttl":59`, wantStatus: http.StatusBadRequest},
		{name: "above_maximum", ttlJSON: `,"ttl":86401`, wantStatus: http.StatusBadRequest},
		{name: "fraction", ttlJSON: `,"ttl":60.5`, wantStatus: http.StatusBadRequest},
		{name: "string", ttlJSON: `,"ttl":"auto"`, wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := setupCloudflareLogicDB(t)
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(response.ErrorHandlerMiddleware())
			router.POST("/groups", CreateGroupHandler)
			body := `{"name":"custom","target_mode":"custom","record_type":"CNAME","record_content":"origin.example.com"` + tt.ttlJSON + `}`
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("POST /groups(%s) status = %d, body = %s, want %d", tt.name, recorder.Code, recorder.Body.String(), tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				groups, err := repository.ListCFPointingGroups(ctx)
				if err != nil || len(groups) != 1 {
					t.Errorf("POST /groups(%s) persisted groups = %d, error = %v, want 1 existing group", tt.name, len(groups), err)
				}
				return
			}
			var envelope struct {
				Data GroupItem `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode POST /groups(%s) error = %v", tt.name, err)
			}
			if envelope.Data.TTL != tt.wantTTL {
				t.Errorf("POST /groups(%s).ttl = %d, want %d", tt.name, envelope.Data.TTL, tt.wantTTL)
			}
			stored, err := repository.GetCFPointingGroup(ctx, envelope.Data.ID)
			if err != nil {
				t.Fatalf("GetCFPointingGroup(%d) error = %v", envelope.Data.ID, err)
			}
			if stored.TTL != tt.wantTTL {
				t.Errorf("persisted POST /groups(%s).ttl = %d, want %d", tt.name, stored.TTL, tt.wantTTL)
			}
		})
	}
}

func TestUpdateGroupTTLAndReadback(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	state, err := repository.GetCFPointingMemberContext(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberContext(%d) error = %v", memberID, err)
	}
	legacy, err := GetGroup(ctx, state.Group.ID)
	if err != nil || legacy.Group.TTL != 300 {
		t.Fatalf("GetGroup(legacy) = %+v, error = %v, want ttl 300", legacy, err)
	}
	dispatchCount := 0
	restoreDispatch := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) {
		dispatchCount++
		return "ttl-task", nil
	})
	t.Cleanup(restoreDispatch)
	ttl := 120
	input := GroupInput{Name: state.Group.Name, PrimaryNodeID: state.Group.PrimaryNodeID, Enabled: true, TTL: &ttl}
	if err := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{"sync_status": model.CFMemberSyncOK}); err != nil {
		t.Fatalf("UpdateCFPointingMemberColumns(%d) error = %v", memberID, err)
	}
	updated, err := UpdateGroup(ctx, state.Group.ID, input)
	if err != nil || updated.TTL != 120 {
		t.Fatalf("UpdateGroup(ttl=120) = %+v, error = %v, want ttl 120", updated, err)
	}
	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID(%d) error = %v", memberID, err)
	}
	if member.SyncStatus != model.CFMemberSyncPending || dispatchCount != 1 {
		t.Errorf("UpdateGroup(ttl=120) member status = %s, dispatches = %d, want pending and 1", member.SyncStatus, dispatchCount)
	}
	// 旧客户端省略新增字段时，不应将已设置的手动 TTL 重置为自动。
	input.TTL = nil
	if _, err := UpdateGroup(ctx, state.Group.ID, input); err != nil {
		t.Fatalf("UpdateGroup(omitted ttl) error = %v", err)
	}
	detail, err := GetGroup(ctx, state.Group.ID)
	if err != nil || detail.Group.TTL != 120 {
		t.Fatalf("GetGroup(after omitted ttl) = %+v, error = %v, want ttl 120", detail, err)
	}
	groups, err := ListGroups(ctx)
	if err != nil || len(groups) != 1 || groups[0].TTL != 120 {
		t.Errorf("ListGroups() = %+v, error = %v, want one group with ttl 120", groups, err)
	}
	invalidTTL := 0
	input.TTL = &invalidTTL
	if _, err := UpdateGroup(ctx, state.Group.ID, input); err == nil {
		t.Fatal("UpdateGroup(ttl=0) error = nil, want validation error")
	}
	stored, err := repository.GetCFPointingGroup(ctx, state.Group.ID)
	if err != nil || stored.TTL != 120 || dispatchCount != 2 {
		t.Errorf("UpdateGroup(invalid ttl) stored = %+v, error = %v, dispatches = %d, want ttl 120 and 2", stored, err, dispatchCount)
	}
}

func TestReconcileMemberGroupTTL(t *testing.T) {
	tests := []struct {
		name       string
		targetMode string
		recordType string
		content    string
		ttl        int
		proxied    bool
		wantTTL    int
	}{
		{name: "node_manual", targetMode: "node", recordType: "A", content: "203.0.113.10", ttl: 120, wantTTL: 120},
		{name: "cname_manual", targetMode: "custom", recordType: "CNAME", content: "origin.example.com", ttl: 60, wantTTL: 60},
		{name: "a_manual", targetMode: "custom", recordType: "A", content: "203.0.113.20", ttl: 600, wantTTL: 600},
		{name: "aaaa_manual", targetMode: "custom", recordType: "AAAA", content: "2001:db8::1", ttl: 86400, wantTTL: 86400},
		{name: "cname_auto", targetMode: "custom", recordType: "CNAME", content: "origin.example.com", ttl: 1, wantTTL: 1},
		{name: "cname_legacy", targetMode: "custom", recordType: "CNAME", content: "origin.example.com", wantTTL: 300},
		{name: "cname_proxied", targetMode: "custom", recordType: "CNAME", content: "origin.example.com", ttl: 120, proxied: true, wantTTL: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, memberID := setupCloudflareLogicDB(t)
			state, err := repository.GetCFPointingMemberContext(ctx, memberID)
			if err != nil {
				t.Fatalf("GetCFPointingMemberContext(%d) error = %v", memberID, err)
			}
			group := state.Group
			group.TargetMode, group.RecordType, group.RecordContent, group.TTL = tt.targetMode, tt.recordType, tt.content, tt.ttl
			if err := repository.SaveCFPointingGroup(ctx, &group); err != nil {
				t.Fatalf("SaveCFPointingGroup(%s) error = %v", tt.name, err)
			}
			if err := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{"proxied": tt.proxied}); err != nil {
				t.Fatalf("UpdateCFPointingMemberColumns(%s) error = %v", tt.name, err)
			}
			fake := &fakeClient{}
			t.Cleanup(SetClientFactoryForTest(func(string) Client { return fake }))
			if err := ReconcileMember(ctx, memberID); err != nil {
				t.Fatalf("ReconcileMember(%s) error = %v", tt.name, err)
			}
			if fake.created == nil {
				t.Fatal("ReconcileMember() did not create a DNS record")
			}
			if got := *fake.created; got.TTL != tt.wantTTL || got.Type != tt.recordType || got.Content != tt.content || got.Proxied != tt.proxied {
				t.Errorf("ReconcileMember(%s) input = %+v, want type=%s content=%s proxied=%t ttl=%d", tt.name, got, tt.recordType, tt.content, tt.proxied, tt.wantTTL)
			}
		})
	}
}

type cachedTTLClient struct {
	fakeClient
}

func (client *cachedTTLClient) GetRecord(_ context.Context, _, recordID string) (*DNSRecord, error) {
	return &DNSRecord{ID: recordID, Type: "A", TTL: 300}, nil
}

func TestReconcileMemberUpdatesGroupTTL(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cached_%t", cached), func(t *testing.T) {
			ctx, memberID := setupCloudflareLogicDB(t)
			state, err := repository.GetCFPointingMemberContext(ctx, memberID)
			if err != nil {
				t.Fatalf("GetCFPointingMemberContext(%d) error = %v", memberID, err)
			}
			state.Group.TTL = 180
			if err := repository.SaveCFPointingGroup(ctx, &state.Group); err != nil {
				t.Fatalf("SaveCFPointingGroup(ttl=180) error = %v", err)
			}
			if err := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{"proxied": false}); err != nil {
				t.Fatalf("UpdateCFPointingMemberColumns(%d) error = %v", memberID, err)
			}
			fake := &cachedTTLClient{fakeClient: fakeClient{records: []DNSRecord{{ID: "existing", Type: "A", TTL: 300}}}}
			if cached {
				if err := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{"cf_record_id": "existing"}); err != nil {
					t.Fatalf("UpdateCFPointingMemberColumns(cached) error = %v", err)
				}
			}
			t.Cleanup(SetClientFactoryForTest(func(string) Client { return fake }))
			if err := ReconcileMember(ctx, memberID); err != nil {
				t.Fatalf("ReconcileMember(cached=%t) error = %v", cached, err)
			}
			if fake.updated == nil || fake.updated.TTL != 180 || fake.created != nil {
				t.Errorf("ReconcileMember(cached=%t) updated = %+v, created = %+v, want update ttl=180 only", cached, fake.updated, fake.created)
			}
		})
	}
}
