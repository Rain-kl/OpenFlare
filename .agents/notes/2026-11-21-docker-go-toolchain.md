# Docker 构建的 Go 工具链

- Canary 镜像的 amd64 和 arm64 任务都在 `go mod download` 阶段失败。`go.mod` 要求 Go 1.26.2，Docker 构建阶段用的是 Go 1.26.0，官方构建镜像设置了 `GOTOOLCHAIN=local`。
- Server 完整镜像、独立后端、跨平台构建、Relay 和 Flared 共用根目录的 `go.mod`。它们用官方 Golang 镜像启动 Go，并在安装依赖前通过 `docker/go-toolchain.sh` 把 `go.mod` 的版本写入用户级 Go 配置。外层 `GOTOOLCHAIN` 留空，避免官方镜像的默认配置覆盖所选版本。
- GitHub Actions、Docker Compose 和 `make cross-build` 走同一个 Dockerfile 内的工具链选择步骤。对 Go 版本的更新只改 `go.mod`，跨平台 Dockerfile 仍保留 Alpine 版本参数。
