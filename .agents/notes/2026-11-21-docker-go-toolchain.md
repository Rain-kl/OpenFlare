# Docker 构建的 Go 工具链

- Canary 镜像的 amd64 和 arm64 任务都在 `go mod download` 阶段失败。`go.mod` 要求 Go 1.26.2，Docker 构建阶段用的是 Go 1.26.0，官方构建镜像设置了 `GOTOOLCHAIN=local`。
- Server 完整镜像、独立后端、跨平台构建、Relay 和 Flared 共用根目录的 `go.mod`。升级其 Go 版本要求时，同步更新这五个 Dockerfile 的构建镜像，并核对镜像标签仍提供 amd64 和 arm64。
