// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package httppool

import (
	"testing"
	"time"
)

func TestDefaultTransport(t *testing.T) {
	tr1 := DefaultTransport()
	if tr1 == nil {
		t.Fatal("DefaultTransport() returned nil")
	}

	tr2 := DefaultTransport()
	if tr1 != tr2 {
		t.Error("DefaultTransport() did not return a singleton instance")
	}
}

func TestNewClient(t *testing.T) {
	timeout := 15 * time.Second
	client := NewClient(timeout)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.Timeout != timeout {
		t.Errorf("NewClient() timeout = %v, want %v", client.Timeout, timeout)
	}

	if client.Transport != DefaultTransport() {
		t.Error("NewClient() is not configured with the default transport")
	}
}
