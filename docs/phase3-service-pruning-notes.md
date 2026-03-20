# 阶段三服务层裁剪说明

本文件补充说明“阶段三：后端接口裁剪与 MVC 收敛”第二轮服务层收敛结果。

## 当前已完成

* 主程序启动链路已不再初始化 GeoIP，也不再启动观测数据自动清理调度。
* `option` 控制器已移除 GeoIP、OpenResty、数据库观测清理相关校验依赖，配置入口只面向模板保留能力。
* `middleware/agent-auth.go` 已删除，Agent 鉴权中间件不再属于模板工程运行时组成部分。
* 已删除一整批没有任何路由入口的服务实现与测试，包括：
  * 节点与 Agent
  * 配置版本分发
  * OpenResty 反向代理
  * TLS 证书与托管域名
  * 访问日志与观测分析
  * GeoIP 查询
  * 观测数据清理

## 当前 service 目录保留范围

当前 `openflare_server/service` 目录仅保留服务端升级相关实现：

* `update.go`
* `update_restart_unix.go`
* `update_restart_windows.go`
* 对应升级测试文件

## 当前剩余风险

* 升级模块内部仍包含部分 OpenFlare 命名、仓库地址和二进制命名约定，后续需要继续模板化。
* `model/` 层中仍保留部分历史 OpenFlare 数据对象，虽然已经没有后端接口入口，但还需要在后续阶段继续删除或迁移。
* 用户、文件、配置等保留模块目前仍主要停留在旧目录结构，尚未完成 `handler/service/repository/model/dto` 的 MVC 收敛。

## 下一步建议

* 开始按模板保留模块拆分 `controller` 与 `model` 之间的直接耦合，逐步引入 `service/repository/dto` 分层。
* 清理 `model/` 中剩余的历史 OpenFlare 数据模型和相关测试。
* 进入前端同步裁剪阶段前，先确认 Swagger、README 和部署说明是否仍残留已删除接口描述。
