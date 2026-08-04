// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import "testing"

func TestSyncMemberTaskHandlerRejectsTrailingJSONContent(t *testing.T) {
	handler := &SyncMemberTaskHandler{}
	for _, payload := range [][]byte{
		[]byte(`{"member_id":7} {}`),
		[]byte(`{"member_id":7}}`),
		[]byte(`{"member_id":7}]`),
	} {
		if normalized, err := handler.ValidatePayload(payload); err == nil {
			t.Errorf("ValidatePayload(%s) = %s, nil; want non-nil error", payload, normalized)
		}
	}
}
