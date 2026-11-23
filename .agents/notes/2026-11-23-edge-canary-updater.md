# Edge 二进制预发布更新

Server 与 Edge 共用 GitHub Releases。`update-openflare-edge.sh canary` 从发行列表中选择 `edge-v` 标签、`draft: false` 和 `prerelease: true` 的发行版，并以 `published_at` 判断最近发布的版本。查询遍历所有分页，因为发行列表顺序与发布时间可能不同。

`canary` 与 `--version` 互斥。省略参数时仍选择最新正式版。各入口共用按架构下载、校验与停止服务、替换二进制、启动服务的流程。
