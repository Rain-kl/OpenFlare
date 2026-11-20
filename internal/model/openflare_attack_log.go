// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// OpenFlareAttackEvent groups matching requests from one source to one host in a time window.
type OpenFlareAttackEvent struct {
	RemoteAddr string `json:"remote_addr"`
	Host       string `json:"host"`
	BucketUnix int64  `json:"bucket_unix"`
	FirstUnix  int64  `json:"first_unix"`
	LastUnix   int64  `json:"last_unix"`
	Count      int64  `json:"count"`
}
