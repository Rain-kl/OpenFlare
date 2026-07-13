// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"encoding/json"
	"fmt"
)

// MaxWAFIPGroupSnapshotBytes is the maximum serialized size accepted for the
// complete Agent/OpenResty WAF IP group runtime document.
const MaxWAFIPGroupSnapshotBytes = 20 << 20

type wafIPGroupSnapshot struct {
	Groups map[string]WAFIPGroup `json:"groups"`
}

// MarshalWAFIPGroupSnapshot serializes the exact document written by the
// Agent to waf_ip_groups.json.
func MarshalWAFIPGroupSnapshot(groups map[string]WAFIPGroup) ([]byte, error) {
	if groups == nil {
		groups = map[string]WAFIPGroup{}
	}
	return json.Marshal(wafIPGroupSnapshot{Groups: groups})
}

// ValidateWAFIPGroupSnapshotSize rejects a complete runtime document that
// cannot be published safely to the OpenResty shared-memory snapshot.
func ValidateWAFIPGroupSnapshotSize(groups map[string]WAFIPGroup) error {
	data, err := MarshalWAFIPGroupSnapshot(groups)
	if err != nil {
		return err
	}
	if len(data) > MaxWAFIPGroupSnapshotBytes {
		return fmt.Errorf("WAF IP 组快照大小 %d 字节超过上限 %d 字节", len(data), MaxWAFIPGroupSnapshotBytes)
	}
	return nil
}
