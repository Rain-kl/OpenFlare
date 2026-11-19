# Pingora 缓存键的上游标识

OCI-JP-A1 的 Pingora 暂存实例拉取 `pingora-20261119-004` 后拒绝应用。该快照的缓存键模板为 `$scheme$proxy_host$request_uri`，Pingora 当时未识别 `$proxy_host`。

OpenResty 将 `$proxy_host` 展开成 `proxy_pass` 配置中的上游组名或地址。Pingora 从同一份路由快照推导该值。上游组名在多上游之间保持稳定，回源请求的 Host 覆盖不参与变量取值。此修复仅更新 Edge，主控可以继续下发已经发布的 v3 快照。
