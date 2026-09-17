// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

import "context"

// PublicConfigProvider supplies GET /api/v1/config/public.
// The owner of w_system_configs (admin) must provide this. The payload is a
// flat key/value map of visibility=1 rows; the frontend reads keys such as
// cap_login_enabled directly off data.
type PublicConfigProvider interface {
	PublicConfig(ctx context.Context) (map[string]string, error)
}
