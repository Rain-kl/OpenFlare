// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"strings"
	"testing"
)

func TestMarshalWAFIPGroupSnapshotMatchesAgentRuntimeDocument(t *testing.T) {
	data, err := MarshalWAFIPGroupSnapshot(map[string]WAFIPGroup{
		"7": {ID: 7, Name: "deny", Type: "manual", Enabled: true, IPList: []string{"192.0.2.7"}, Checksum: "sum"},
	})
	if err != nil {
		t.Fatalf("MarshalWAFIPGroupSnapshot failed: %v", err)
	}
	want := `{"groups":{"7":{"id":7,"name":"deny","type":"manual","enabled":true,"ip_list":["192.0.2.7"],"checksum":"sum"}}}`
	if string(data) != want {
		t.Fatalf("snapshot = %s, want %s", data, want)
	}
}

func TestValidateWAFIPGroupSnapshotSizeBoundary(t *testing.T) {
	groups := map[string]WAFIPGroup{
		"1": {ID: 1, Type: "manual", Enabled: true, IPList: []string{"192.0.2.1"}, Checksum: strings.Repeat("a", 64)},
	}
	base, err := MarshalWAFIPGroupSnapshot(groups)
	if err != nil {
		t.Fatalf("marshal base snapshot: %v", err)
	}
	groups["1"] = WAFIPGroup{
		ID:       1,
		Name:     strings.Repeat("x", MaxWAFIPGroupSnapshotBytes-len(base)),
		Type:     "manual",
		Enabled:  true,
		IPList:   []string{"192.0.2.1"},
		Checksum: strings.Repeat("a", 64),
	}
	atLimit, err := MarshalWAFIPGroupSnapshot(groups)
	if err != nil {
		t.Fatalf("marshal boundary snapshot: %v", err)
	}
	if len(atLimit) != MaxWAFIPGroupSnapshotBytes {
		t.Fatalf("boundary snapshot size = %d, want %d", len(atLimit), MaxWAFIPGroupSnapshotBytes)
	}
	if err := ValidateWAFIPGroupSnapshotSize(groups); err != nil {
		t.Fatalf("boundary snapshot rejected: %v", err)
	}

	group := groups["1"]
	group.Name += "x"
	groups["1"] = group
	if err := ValidateWAFIPGroupSnapshotSize(groups); err == nil {
		t.Fatal("oversized snapshot was accepted")
	}
}
