// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

const (
	errInvalidParams       = "无效的参数"
	errOptionInitFailed    = "系统选项初始化失败"
	errGeoIPProvider       = "归属方式仅支持 disabled、mmdb、ip-api、geojs、ipinfo"
	errGeoIPIPEmpty        = "IP 不能为空"
	errGeoIPIPInvalid      = "IP 格式无效"
	errGeoIPLookupDisabled = "GeoIP 查询已禁用"
)
