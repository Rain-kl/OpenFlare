# 阶段三后端接口裁剪说明

本文件补充说明“阶段三：后端接口裁剪与 MVC 收敛”当前已经完成的第一轮接口收敛结果。

## 当前已移除的后端接口组

以下接口组已经从 `openflare_server/router/api-router.go` 中移除，不再对外暴露：

* `agent`
* `nodes`
* `apply-logs`
* `access-logs`
* `dashboard`
* `config-versions`
* `proxy-routes`
* `managed-domains`
* `tls-certificates`
* `option/geoip/lookup`
* `option/database/cleanup`

## 当前已删除的 Controller 入口

以下控制器文件已删除，用于同步收缩 Swagger 注解与 HTTP 入口：

* `openflare_server/controller/access_log.go`
* `openflare_server/controller/agent.go`
* `openflare_server/controller/config_version.go`
* `openflare_server/controller/dashboard.go`
* `openflare_server/controller/database.go`
* `openflare_server/controller/geoip.go`
* `openflare_server/controller/managed_domain.go`
* `openflare_server/controller/node.go`
* `openflare_server/controller/proxy_route.go`
* `openflare_server/controller/tls_certificate.go`

## 当前保留的接口主线

当前 `api-router` 保留的模板能力主要包括：

* 用户与认证
* 邮箱验证与密码重置
* 系统配置
* 文件上传与文件管理
* 服务端版本升级

## 下一步建议

* 继续裁剪 `service/` 中仍然围绕节点、Agent、配置分发、OpenResty、观测分析的实现文件。
* 将保留模块继续按 `handler/service/repository/model/dto` 职责拆分，避免 controller 直接承接过多流程逻辑。
* 后续前端阶段需要同步删除对应页面、导航、API client、类型定义和测试用例，保持接口与界面同批次收敛。
