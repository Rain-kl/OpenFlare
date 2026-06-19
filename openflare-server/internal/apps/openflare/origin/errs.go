// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package origin

const (
	errOriginAddressRequired  = "源站地址不能为空"
	errOriginAddressInvalid   = "源站地址格式不合法"
	errOriginAddressExists    = "源站地址已存在"
	errOriginDeleteReferenced = "该源站仍被规则引用，无法删除"
	errOriginMissingPort      = "源站地址缺少端口"
	errOriginNotFound         = "源站不存在"
)
