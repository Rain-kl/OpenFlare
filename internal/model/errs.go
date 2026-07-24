// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// Domain validation messages used by model.Validate and other no-IO rules.
// Persistence / data-access messages belong in internal/repository (do not import repository).
const (
	errTemplateKeyRequired                 = "模板标识符不能为空"
	errTemplateNameRequired                = "模板名称不能为空"
	errTemplateContentRequired             = "模板内容不能为空"
	errAuthSourceNameRequired              = "认证源名称不能为空"
	errAuthSourceNameInvalid               = "认证源名称只能包含字母、数字、短横线或下划线，且必须以字母或数字开头"
	errAuthSourceTypeUnsupported           = "认证源类型仅支持 oidc"
	errAuthSourceDiscoveryURLRequired      = "OIDC 认证源必须配置 Discovery URL"
	errAuthSourceClientCredentialsRequired = "启用认证源前必须配置 Client ID 和 Client Secret" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
)
