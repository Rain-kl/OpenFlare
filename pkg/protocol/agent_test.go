package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAgentProtocolJSONTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		value    any
		expected map[string]string
	}{
		{
			name:  "NodePayload",
			value: NodePayload{},
			expected: map[string]string{
				"NodeID": "node_id",
				"Name":   "name",
			},
		},
		{
			name:  "AgentSettings",
			value: AgentSettings{},
			expected: map[string]string{
				"HeartbeatInterval":       "heartbeat_interval",
				"WebsocketUpgradeEnabled": "websocket_upgrade_enabled",
				"RestartOpenrestyNow":     "restart_openresty_now",
			},
		},
		{
			name:  "WSMessage",
			value: WSMessage{},
			expected: map[string]string{
				"Type":    "type",
				"Payload": "payload,omitempty",
			},
		},
		{
			name:  "RegisterNodeResponse",
			value: RegisterNodeResponse{},
			expected: map[string]string{
				"NodeID":      "node_id",
				"AccessToken": "agent_token",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			typ := reflect.TypeOf(tc.value)
			for field, wantTag := range tc.expected {
				structField, ok := typ.FieldByName(field)
				if !ok {
					t.Fatalf("field %q not found on %s", field, tc.name)
				}
				gotTag := structField.Tag.Get("json")
				if gotTag != wantTag {
					t.Fatalf("field %q json tag = %q, want %q", field, gotTag, wantTag)
				}
			}
		})
	}
}

func TestNodePayloadJSONRoundTrip(t *testing.T) {
	t.Parallel()

	payload := NodePayload{
		NodeID:          "node-1",
		Name:            "edge-a",
		OpenrestyStatus: OpenrestyStatusHealthy,
		HealthEvents:    []NodeHealthEvent{},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded NodePayload
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.NodeID != payload.NodeID || decoded.Name != payload.Name {
		t.Fatalf("round trip mismatch: %+v", decoded)
	}
}
