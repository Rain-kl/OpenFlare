// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
