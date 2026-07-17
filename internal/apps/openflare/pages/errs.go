// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package pages provides logics and management for OpenFlare static page deployments.
package pages

const (
	errPagesProjectNotFound          = "pages 项目不存在"
	errPagesSlugExists               = "pages 项目标识已存在"
	errPagesNameRequired             = "pages 项目名称不能为空"
	errPagesSlugInvalid              = "pages 项目标识只能包含小写字母、数字和连字符"
	errPagesDeleteReferenced         = "pages 项目已被规则引用，不能删除"
	errPagesDeploymentNotFound       = "pages 部署不存在"
	errPagesDeploymentMismatch       = "pages 部署不属于该项目"
	errPagesDeleteActiveDeploy       = "不能删除当前激活的 Pages 部署"
	errPagesPackageMissing           = "缺少 Pages 部署包"
	errPagesPackageURLRequired       = "请填写部署包下载链接"
	errPagesPackageURLInvalid        = "部署包下载链接无效，仅支持 http/https"
	errPagesPackageURLDownloadFailed = "从链接下载部署包失败"
	errPagesPackageURLTooLarge       = "链接指向的部署包超过大小限制"
	errPagesPackageNotZip            = "pages 部署包必须是 .zip 文件" // legacy alias kept for tests
	errPagesPackageUnsupported       = "pages 部署包仅支持 zip、tar.gz、tar.xz、tar.bz2、tar、7z 格式"
	errPagesPackageInvalidZip        = "pages 部署包不是有效 zip 文件" // legacy alias
	errPagesPackageInvalid           = "pages 部署包不是有效的压缩文件"
	errPagesPackageEmpty             = "pages 部署包不能为空"
	errPagesPackageExtractedTooLarge = "pages 部署包展开后体积超过限制"
	errPagesPackageFileTooLarge      = "pages 部署包内文件过大"
	errPagesAPIProxyPathRequired     = "启用 API 反代时，匹配路径不能为空"
	errPagesAPIProxyPathPrefix       = "API 反代匹配路径必须以 '/' 开头"
	errPagesAPIProxyPassRequired     = "启用 API 反代时，后端服务地址不能为空"             //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errPagesAPIProxyPassInvalid      = "API 反代后端服务地址必须是有效的 HTTP/HTTPS URL" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errPagesPackagePathEmpty         = "pages 部署包路径为空"
	errPagesPackageUploadMissing     = "pages 部署包上传记录不存在"
	errPagesPackageNotInActiveConfig = "pages 部署尚未进入激活配置"
	errPagesDeploymentHashMissing    = "pages 部署包哈希缺失"
	errPagesInvalidSnapshotFormat    = "配置快照格式无效"
)
