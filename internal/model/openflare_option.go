// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// OpenFlareOption is the key/value DTO used by the OpenFlare option API surface.
// Key 与 w_system_configs.key 一致（snake_case）。此类型仅作为
// /api/v1/d/option 接口的请求/响应载体，不再映射到独立的 of_options 表。
type OpenFlareOption struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StartTime records process start time (seconds) for the public /status endpoint.
var StartTime = time.Now().Unix()
