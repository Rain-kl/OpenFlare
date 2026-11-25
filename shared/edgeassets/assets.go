// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package edgeassets embeds the HTML templates and static assets shared by the
// OpenFlare control plane, the Rust edge runtime, and the frontend previews.
package edgeassets

import _ "embed"

// BrandMark is the inline SVG used by server-rendered and edge-rendered pages.
//
//go:embed pow_static/img/openflare-mark.svg
var BrandMark string

// ClassicPoWPage is the built-in classic proof-of-work challenge page.
//
//go:embed pow_templates/classic.html
var ClassicPoWPage string

// ShieldPoWPage is the built-in shield proof-of-work challenge page.
//
//go:embed pow_templates/shield.html
var ShieldPoWPage string

// SiteAuthPage is the default site authentication login page.
//
//go:embed site_auth.html
var SiteAuthPage string
