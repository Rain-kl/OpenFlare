# Pingora 与 Go Agent 并行开发

- Go Agent/OpenResty 继续作为现有发布线。独立的 Rust/Pingora 节点在一个进程中承担注册、同步、代理和上报，直接连接现有主控。进程管理留在节点本地。
- 主控沿用节点协议和业务资源，通过独立的 Pingora 发布组与配置契约限制试点范围。路由和策略使用本地快照热更新，进程退出后从最后有效版本恢复。
- Rust 产物独立编号并使用独立发行源。主仓库 Go 更新器依赖 `releases/latest` 和发行版列表，现有清理流程扫描发行版列表。两个产品混用发行源会影响选包和清理。
- HTTP/3 不作为首版条件。缓存跟随 Pingora 上游接口，支持范围通过节点能力声明。

实现和验收以 [节点设计](../../docs/design/pingora-edge-design.md) 和 [开发说明](../../docs/design/pingora-edge-development.md) 为准。
