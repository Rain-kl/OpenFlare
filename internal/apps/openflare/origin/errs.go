// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package origin defines shared error messages for origin management.
package origin

const (
	errOriginAddressRequired  = "origin address must not be empty"
	errOriginAddressInvalid   = "invalid origin address format"
	errOriginAddressExists    = "origin address already exists"
	errOriginDeleteReferenced = "this origin is still referenced by rules and cannot be deleted"
	errOriginMissingPort      = "origin address is missing a port"
	errOriginNotFound         = "Origin was not found"
)
