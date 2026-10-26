// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegisterNodeResponseTokenAliases(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "canonical", body: `{"node_id":"node-1","agent_token":"issued"}`, want: "issued"},
		{name: "legacy", body: `{"node_id":"node-1","access_token":"issued"}`, want: "issued"},
		{name: "matching aliases", body: `{"agent_token":"issued","access_token":"issued"}`, want: "issued"},
		{name: "conflicting aliases", body: `{"agent_token":"a","access_token":"b"}`, wantErr: true},
		{name: "invalid JSON", body: `{`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got RegisterNodeResponse
			err := json.Unmarshal([]byte(tt.body), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal(%s) error = %v, want error %t", tt.name, err, tt.wantErr)
			}
			if got.AccessToken != tt.want {
				t.Errorf("Unmarshal(%s).AccessToken = %q, want %q", tt.name, got.AccessToken, tt.want)
			}
		})
	}
}

func TestRegistrationRequestKeepsRetryCredentialOutOfTelemetry(t *testing.T) {
	request := RegisterNodeRequest{NodePayload: NodePayload{NodeID: "node-1"}, RegistrationKey: "private"}
	registration, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(registration), `"registration_key":"private"`) {
		t.Errorf("Marshal(registration) = %s, want retry credential", registration)
	}
	telemetry, err := json.Marshal(request.NodePayload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(telemetry), "registration_key") || strings.Contains(string(telemetry), "private") {
		t.Errorf("Marshal(heartbeat) = %s, want no enrollment credential", telemetry)
	}
}
