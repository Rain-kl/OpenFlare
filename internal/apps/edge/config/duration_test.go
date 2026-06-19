package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMillisecondDurationUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "null", input: "null", want: 0},
		{name: "empty", input: `""`, want: 0},
		{name: "integer milliseconds", input: "30000", want: 30 * time.Second},
		{name: "duration string", input: `"5s"`, want: 5 * time.Second},
		{name: "invalid number", input: "not-a-number", wantErr: true},
		{name: "invalid duration string", input: `"not-a-duration"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d MillisecondDuration
			err := json.Unmarshal([]byte(tt.input), &d)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Duration() != tt.want {
				t.Fatalf("got %s, want %s", d, tt.want)
			}
		})
	}
}

func TestMillisecondDurationMarshalJSON(t *testing.T) {
	d := MillisecondDuration(7 * time.Second)
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if string(data) != "7000" {
		t.Fatalf("unexpected marshaled value: %s", string(data))
	}
}