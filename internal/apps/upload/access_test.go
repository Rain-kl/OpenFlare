// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"testing"
)

func TestGetActiveRequiresUploadID(t *testing.T) {
	_, err := GetActiveUpload(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for empty upload id")
	}
}
