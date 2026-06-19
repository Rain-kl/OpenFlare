// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

const (
	errPagesProjectNotFound          = "Pages 项目不存在"
	errPagesSlugExists               = "Pages 项目标识已存在"
	errPagesNameRequired             = "Pages 项目名称不能为空"
	errPagesSlugInvalid              = "Pages 项目标识只能包含小写字母、数字和连字符"
	errPagesDeleteReferenced         = "Pages 项目已被规则引用，不能删除"
	errPagesDeploymentNotFound       = "Pages 部署不存在"
	errPagesDeploymentMismatch       = "Pages 部署不属于该项目"
	errPagesDeleteActiveDeploy       = "不能删除当前激活的 Pages 部署"
	errPagesPackageMissing           = "缺少 Pages 部署包"
	errPagesPackageNotZip            = "Pages 部署包必须是 .zip 文件"
	errPagesPackageInvalidZip        = "Pages 部署包不是有效 zip 文件"
	errPagesPackageEmpty             = "Pages 部署包不能为空"
	errPagesAPIProxyPathRequired     = "启用 API 反代时，匹配路径不能为空"
	errPagesAPIProxyPathPrefix       = "API 反代匹配路径必须以 '/' 开头"
	errPagesAPIProxyPassRequired     = "启用 API 反代时，后端服务地址不能为空"
	errPagesAPIProxyPassInvalid      = "API 反代后端服务地址必须是有效的 HTTP/HTTPS URL"
	errPagesPackagePathEmpty         = "Pages 部署包路径为空"
	errPagesPackageUploadMissing     = "Pages 部署包上传记录不存在"
	errPagesPackageNotInActiveConfig = "Pages 部署尚未进入激活配置"
	errPagesInvalidSnapshotFormat    = "配置快照格式无效"
)
