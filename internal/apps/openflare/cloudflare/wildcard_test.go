// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func TestWildcardMemberDNSRecords(t *testing.T) {
	tests := []struct {
		name, mode, recordType, content string
		proxied                         bool
	}{
		{name: "custom_A", mode: model.CFPointingTargetModeCustom, recordType: "A", content: "203.0.113.20"},
		{name: "custom_AAAA", mode: model.CFPointingTargetModeCustom, recordType: "AAAA", content: "2001:db8::20", proxied: true},
		{name: "custom_CNAME", mode: model.CFPointingTargetModeCustom, recordType: "CNAME", content: "edge.example.net", proxied: true},
		{name: "node_A", recordType: "A", content: "203.0.113.10", proxied: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, originalID := setupCloudflareLogicDB(t)
			state, err := repository.GetCFPointingMemberContext(ctx, originalID)
			if err != nil {
				t.Fatal(err)
			}
			state.Group.TargetMode, state.Group.RecordType, state.Group.RecordContent = tt.mode, tt.recordType, tt.content
			state.Group.DefaultProxied = tt.proxied
			if err := repository.SaveCFPointingGroup(ctx, &state.Group); err != nil {
				t.Fatal(err)
			}
			if tt.mode != model.CFPointingTargetModeCustom {
				if err := repository.UpdateOpenFlareNodeColumns(ctx, &state.Node, map[string]any{"ip": tt.content}); err != nil {
					t.Fatal(err)
				}
			}
			domain := &model.ZoneDomain{ZoneID: state.Zone.ID, Domain: "*.s3.example.com"}
			if err := repository.CreateZoneDomain(ctx, domain); err != nil {
				t.Fatal(err)
			}
			available, err := ListAvailableDomains(ctx)
			if err != nil || len(available) != 1 || available[0].Domain != domain.Domain {
				t.Fatalf("ListAvailableDomains() = %+v, %v, want wildcard domain", available, err)
			}
			t.Cleanup(SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) { return "task-1", nil }))
			member, err := CreateMember(ctx, state.Group.ID, MemberCreateInput{ZoneDomainID: domain.ID})
			if err != nil {
				t.Fatalf("CreateMember(wildcard) error = %v", err)
			}
			var creates, updates, deletes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/zones":
					if got := r.URL.Query().Get("name"); got != "example.com" {
						t.Errorf("FindZone name = %q, want example.com", got)
					}
					writeCFTestResponse(t, w, []Zone{{ID: "zone-1", Name: "example.com"}})
				case "/zones/zone-1/dns_records", "/zones/zone-1/dns_records/wildcard-record":
					if (r.Method == http.MethodPut || r.Method == http.MethodDelete) && r.URL.Path != "/zones/zone-1/dns_records/wildcard-record" {
						t.Errorf("DNS %s path = %q, want record ID path", r.Method, r.URL.Path)
						w.WriteHeader(http.StatusMethodNotAllowed)
						return
					}
					switch r.Method {
					case http.MethodGet:
						if r.URL.Path == "/zones/zone-1/dns_records/wildcard-record" {
							writeCFTestResponse(t, w, DNSRecord{ID: "wildcard-record", Name: domain.Domain, Type: tt.recordType})
							return
						}
						if r.URL.Query().Get("name") != domain.Domain || r.URL.Query().Get("type") != tt.recordType {
							t.Errorf("ListRecords query = %v, want wildcard name and %s", r.URL.Query(), tt.recordType)
						}
						writeCFTestResponse(t, w, []DNSRecord{})
					case http.MethodPost, http.MethodPut:
						var input RecordInput
						if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
							t.Errorf("Decode(record) error = %v", err)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						wantTTL := 300
						if tt.proxied {
							wantTTL = 1
						}
						if input.Name != domain.Domain || input.Type != tt.recordType || input.Content != tt.content || input.Proxied != tt.proxied || input.TTL != wantTTL {
							t.Errorf("DNS record input = %+v, want name=%s type=%s content=%s proxied=%t TTL=%d", input, domain.Domain, tt.recordType, tt.content, tt.proxied, wantTTL)
						}
						if r.Method == http.MethodPost {
							creates.Add(1)
						} else {
							updates.Add(1)
						}
						writeCFTestResponse(t, w, DNSRecord{ID: "wildcard-record", Name: input.Name, Type: input.Type, Content: input.Content})
					case http.MethodDelete:
						deletes.Add(1)
						writeCFTestResponse(t, w, map[string]string{"id": "wildcard-record"})
					default:
						t.Errorf("unexpected method %s", r.Method)
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				default:
					t.Errorf("unexpected Cloudflare path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			t.Cleanup(SetClientFactoryForTest(func(token string) Client {
				return NewHTTPClient(token, WithBaseURL(server.URL), WithHTTPClient(server.Client()))
			}))
			for range 2 {
				if err := ReconcileMember(ctx, member.ID); err != nil {
					t.Fatalf("ReconcileMember(wildcard %s) error = %v", tt.recordType, err)
				}
			}
			stored, err := repository.GetCFPointingMemberByID(ctx, member.ID)
			if err != nil || stored.SyncStatus != model.CFMemberSyncOK || stored.CFRecordID != "wildcard-record" {
				t.Fatalf("synced wildcard member = %+v, error=%v", stored, err)
			}
			if err := DeleteManagedRecord(ctx, member.ID); err != nil {
				t.Fatalf("DeleteManagedRecord(wildcard) error = %v", err)
			}
			if creates.Load() != 1 || updates.Load() != 1 || deletes.Load() != 1 {
				t.Errorf("DNS create/update/delete counts = %d/%d/%d, want 1/1/1", creates.Load(), updates.Load(), deletes.Load())
			}
		})
	}
}
