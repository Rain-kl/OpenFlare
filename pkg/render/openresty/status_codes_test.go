// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import "testing"

func TestExpandStatusCodeTags(t *testing.T) {
	t.Parallel()
	codes, err := ExpandStatusCodeTags([]string{"500-502", "522", "501"})
	if err != nil {
		t.Fatal(err)
	}
	// want sorted unique: 500,501,502,522
	if len(codes) != 4 || codes[0] != 500 || codes[3] != 522 {
		t.Fatalf("got %v", codes)
	}
	_, err = ExpandStatusCodeTags([]string{"399"})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = ExpandStatusCodeTags([]string{"503-500"})
	if err == nil {
		t.Fatal("expected reverse range error")
	}
	_, err = ExpandStatusCodeTags([]string{"5xx"})
	if err == nil {
		t.Fatal("expected syntax error")
	}
}
