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
	errPagesActorMissing             = "无法识别当前用户"
	errPagesEntryFileMissing         = "当前激活部署中不存在指定入口文件"
	errPagesSourceNotFound           = "pages 部署源不存在"
	errPagesSourceTypeRequired       = "请选择 pages 部署源类型"
	errPagesSourceTypeUnsupported    = "当前阶段仅支持远程地址部署源"
	errPagesSourceRemoteFields       = "远程地址来源不能包含 GitHub 或自动更新配置"
	errPagesSourceRemoteURLRequired  = "请提供远程部署包地址"
	errPagesSourceRemoteURLMode      = "remote_url_set 与 remote_url 参数不匹配"
	errPagesSourceRemoteURLInvalid   = "远程部署包地址无效，仅支持不含用户信息和片段的 http/https 地址"
	errPagesSourceNetworkPolicy      = "远程地址网络策略仅支持 public 或 trusted_internal"
	errPagesSourceCheckUnsupported   = "远程地址来源不支持检查更新，请使用立即同步"
	errPagesSourceActionBusy         = "pages 部署源任务正在执行"
	errPagesSourceActionInvalid      = "pages 部署源任务参数无效"
	errPagesSourceActionStale        = "pages 部署源配置已变化，本次任务已跳过"
	errPagesSourceLeaseLost          = "pages 部署源任务执行权已失效"
	errPagesSourceSyncFailed         = "pages 部署源同步失败"
	errPagesSourceTaskDispatchFailed = "pages 部署源任务入队失败"
	errPagesSourceInternal           = "pages 部署源操作失败，请稍后重试"
)
