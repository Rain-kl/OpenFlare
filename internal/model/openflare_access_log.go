// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// OpenFlareAccessLogTrafficSummary is a window-level traffic summary from access logs.
type OpenFlareAccessLogTrafficSummary struct {
	RequestCount  int64
	ErrorCount    int64
	UniqueIPCount int64
	BytesSent     int64
	RequestLength int64
	NodeCount     int64
}

// OpenFlareAccessLogValueCount is a dimension value count.
type OpenFlareAccessLogValueCount struct {
	Value string
	Count int64
}

// OpenFlareAccessLogNodeAggregate is per-node traffic over a window.
type OpenFlareAccessLogNodeAggregate struct {
	NodeID        string
	RequestCount  int64
	ErrorCount    int64
	UniqueIPCount int64
}
