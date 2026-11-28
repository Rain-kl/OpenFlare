// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/openflare/dnsprovider"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientVerifyTokenAndManageARecord(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		writeCFTestResponse(t, w, map[string]any{"status": "active"})
	})
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != "example.com" {
			t.Errorf("zone name = %q, want example.com", got)
		}
		writeCFTestResponse(t, w, []map[string]any{{"id": "zone-1", "name": "example.com"}})
	})
	mux.HandleFunc("/zones/zone-1/dns_records", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeCFTestResponse(t, w, []map[string]any{})
		case http.MethodPost:
			var input RecordInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatalf("Decode(create) error = %v", err)
			}
			if input.Type != "A" || input.Name != "api.example.com" || input.Content != "203.0.113.10" || !input.Proxied {
				t.Errorf("create input = %+v", input)
			}
			writeCFTestResponse(t, w, map[string]any{"id": "record-1", "type": "A", "name": input.Name, "content": input.Content, "proxied": input.Proxied})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/zones/zone-1/dns_records/record-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			writeCFTestResponse(t, w, map[string]any{"id": "record-1", "type": "A", "name": "api.example.com", "content": "203.0.113.11", "proxied": false})
		case http.MethodDelete:
			writeCFTestResponse(t, w, map[string]any{"id": "record-1"})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := NewHTTPClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	ctx := context.Background()

	if err := client.VerifyToken(ctx); err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	zone, err := client.FindZone(ctx, "example.com")
	if err != nil || zone.ID != "zone-1" {
		t.Fatalf("FindZone() = %+v, %v", zone, err)
	}
	records, err := client.ListARecords(ctx, zone.ID, "api.example.com")
	if err != nil || len(records) != 0 {
		t.Fatalf("ListARecords() = %+v, %v", records, err)
	}
	record, err := client.CreateARecord(ctx, zone.ID, RecordInput{Type: "A", Name: "api.example.com", Content: "203.0.113.10", Proxied: true, TTL: 1})
	if err != nil || record.ID != "record-1" {
		t.Fatalf("CreateARecord() = %+v, %v", record, err)
	}
	if _, err := client.UpdateARecord(ctx, zone.ID, record.ID, RecordInput{Type: "A", Name: record.Name, Content: "203.0.113.11", TTL: 300}); err != nil {
		t.Fatalf("UpdateARecord() error = %v", err)
	}
	if err := client.DeleteRecord(ctx, zone.ID, record.ID); err != nil {
		t.Fatalf("DeleteRecord() error = %v", err)
	}
}

func writeCFTestResponse(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "errors": []any{}, "result": result}); err != nil {
		t.Fatalf("Encode(response) error = %v", err)
	}
}

func TestDelegationReconcilesNSRecordsInParentZone(t *testing.T) {
	records := []DNSRecord{
		{ID: "keep", Type: "NS", Name: "sdns.example.com", Content: "NS1.EXAMPLE.COM."},
		{ID: "replace", Type: "NS", Name: "sdns.example.com", Content: "old.example.com"},
	}
	var changes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			require.Equal(t, "sdns.example.com", r.URL.Query().Get("name"))
			require.Equal(t, "NS", r.URL.Query().Get("type"))
			writeCFTestResponse(t, w, records)
		case http.MethodPost:
			var in RecordInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&in))
			require.Equal(t, "NS", in.Type)
			require.Equal(t, "ns2.example.com", in.Content)
			require.False(t, in.Proxied)
			record := DNSRecord{ID: "add", Type: in.Type, Name: in.Name, Content: in.Content}
			records = append(records, record)
			changes = append(changes, "create")
			writeCFTestResponse(t, w, record)
		case http.MethodDelete:
			require.Equal(t, "/zones/parent/dns_records/replace", r.URL.Path)
			records = slices.DeleteFunc(records, func(record DNSRecord) bool { return record.ID == "replace" })
			changes = append(changes, "delete")
			writeCFTestResponse(t, w, struct{}{})
		default:
			t.Errorf("unexpected DNS API request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	client := NewHTTPClient("test-token", WithBaseURL(srv.URL))
	hosts := []string{"ns1.example.com", "ns2.example.com"}
	result, err := dnsprovider.ReconcileDelegation(t.Context(), client, Zone{ID: "parent", Name: "example.com"}, "sdns.example.com", hosts, 600)
	require.NoError(t, err)
	require.Equal(t, dnsprovider.DelegationChanges{Created: 1, Removed: 1}, result)
	require.Equal(t, []string{"create", "delete"}, changes)
	result, err = dnsprovider.ReconcileDelegation(t.Context(), client, Zone{ID: "parent", Name: "example.com"}, "sdns.example.com", hosts, 600)
	require.NoError(t, err)
	require.Equal(t, dnsprovider.DelegationChanges{}, result)
	require.Len(t, changes, 2)
}
