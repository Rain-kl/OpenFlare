// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import "errors"

// Validation and conflict errors returned when registering roots and hostnames.
var (
	ErrInvalidZoneRoot     = errors.New(errZoneRootInvalid)
	ErrDomainOutsideZone   = errors.New(errDomainOutsideZone)
	ErrDomainExists        = errors.New(errDomainExists)
	ErrCertificateNotFound = errors.New(errCertificateNotFound)
)

const (
	errZoneRootInvalid     = "zone must be a valid registered root domain"
	errDomainOutsideZone   = "domain does not belong to this zone"
	errZoneNotFound        = "Zone was not found"
	errDomainNotFound      = "Domain was not found"
	errDomainExists        = "domain already exists"
	errCertificateNotFound = "selected certificate was not found"
	errDomainBoundToRoute  = "domain is bound to a proxy route; remove the binding first"
	errZoneHasDomains      = "root domain still contains domains; delete them all first"
	errStatsRangeInvalid   = "invalid time range; select 24h, 7d or 30d"
)
