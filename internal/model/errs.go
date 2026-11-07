// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// Domain validation messages used by model.Validate and other no-IO rules.
// Persistence / data-access messages belong in internal/repository (do not import repository).
const (
	errTemplateKeyRequired                 = "Template identifier must not be empty"
	errTemplateNameRequired                = "Template name must not be empty"
	errTemplateContentRequired             = "Template content must not be empty"
	errAuthSourceNameRequired              = "authentication source name must not be empty"
	errAuthSourceNameInvalid               = "authentication source name may only contain letters, digits, hyphens and underscores, and must start with a letter or digit"
	errAuthSourceTypeUnsupported           = "authentication source type must be oidc"
	errAuthSourceDiscoveryURLRequired      = "OIDC authentication sources require a Discovery URL"
	errAuthSourceClientCredentialsRequired = "client ID and Client Secret are required before enabling an authentication source" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
)
