# SmartDNS CI/CD、一键部署与 Docker 容器化架构说明

## 1. 设计目标与对齐原则

为独立 Rust 权威解析节点 `dns/`（`openflare-dns`）补齐与 `openflare-edge` 对齐的发布交付流水线：
- **版本与身份隔离**：通过编译期 `OPENFLARE_DNS_VERSION` 环境变量覆盖默认 `"dev"` 版本串，既支持 `clap` `--version`，也同步写入 `/metrics` 与 `ready.json`；
- **敏感凭证与状态持久化**：支持 `--token-file` / `--token-type`（`discovery` | `instance`），避免 `ps`/`systemctl status` 命令行暴露令牌；通过 `<cache_dir>/identity.json`（原子 `0600` 写入）在节点重启后保留分配到的 `instance_id`、`instance_token`、`listen_addr` 与 `public_addr`，支持使用 `X-DNS-Token` 恢复会话并在失效时自动回退至 bootstrap token 重注册；
- **就绪探针与优雅退出**：服务监听就绪后写入 `<cache_dir>/ready.json`（含 `pid`、`binary_version`、`listen`、`updated_at`），退出时 via `ReadyFileGuard` 清理；同时监听 `SIGINT` 与 Unix `SIGTERM`（`tokio::signal::unix::signal`）确保容器与 `systemd` 下能干净下线；
- **Release 与镜像流水线解耦**：延续 `build-release -> publish-release / build-image` 的 `workflow_run` 结构，支持 `dns-vX.Y.Z` 正式发布（触发 GitHub Release + `:latest` 多架构镜像）与 `canary` 分支/`workflow_dispatch`（触发 `dns-v<next_patch>-canary.<run_number>` 预发布 + `:canary` 多架构镜像），并在 GitHub Release 设置 `make_latest: "false"` 避免覆盖主服务端 Release；
- **非 root 绑定特权端口**：`systemd` 单元与 Alpine Docker 镜像（UID/GID `10001`）分别通过 `AmbientCapabilities=CAP_NET_BIND_SERVICE` 与 `setcap cap_net_bind_service+ep /usr/local/bin/openflare-dns` 获得绑定 `53` 端口能力；容器默认监听 `5353` 并由宿主机按 `${OPENFLARE_DNS_PORT:-53}:5353/udp|tcp` 映射。

## 2. 验证清单

- 脚本静态检查（`bash -n`、`sh -n`、`--help`）与 YAML 解析（6 个 GitHub Actions YAML 与 `docker-compose.yaml`）；
- Rust `cargo clippy --locked`、`cargo test --locked`（5/5 通过）、带 `OPENFLARE_DNS_VERSION` 注入的 `--locked` 构建 / `--version` / `bench --queries 200 --concurrency 4`；
- Docker 静态与结构检查：`docker compose config -q`、`sh -n` + 模拟 `PATH` 执行 `dns-entrypoint.sh`（验证 `bootstrap-token` 原子写入与 `serve` 参数拼装），以及 `Cargo.lock` 依赖中 `rquickjs-sys 0.10.0` 已包含预编译 `x86_64`/`aarch64-unknown-linux-musl` 绑定（无需 `bindgen`）；注：当前构建沙盒直连 `registry-1.docker.io` 出现 TLS EOF 且本地无 `rust:1.95-alpine3.23`/`alpine:3.23` 基础镜像缓存，实机 `docker build` 需在具备 Docker Hub / 镜像源可达性的主机或 GitHub Actions 中完成；
- Go 控制面 `golangci-lint`（0 issues）与 `go test -race` 单测，前端 `merge-i18n-fragments`、`check:i18n`（6080 keys, 3 locales）、`biome`、`tsc --noEmit`、`eslint --max-warnings 0`、Vitest 命令生成测试（2/2 通过）。
