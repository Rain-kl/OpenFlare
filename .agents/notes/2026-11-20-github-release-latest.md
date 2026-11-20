# GitHub Releases 的 Latest 归属

Server 与 Edge 共用一份 GitHub 发行列表。Server 发布工作流校验 `vX.Y.Z` 系列标签，只将正式版标为 Latest。Server、Relay 和隧道节点的构建任务通过 GitHub CLI 上传各自产物，发行元数据由创建步骤统一设置。

Edge 的正式版和预览版发布均显式设置 `make_latest: false`。Edge 安装脚本、控制端和 Pingora 自动更新通过发行列表匹配 `edge-v*`。Edge 容器按独立镜像仓库的标签分发。

旧版工作流曾标记 Edge 为 Latest 时，可用 `gh release edit <Server 正式版标签> --latest` 修正现有发行版。默认分支中的 Edge 发布工作流决定后续发版行为。
