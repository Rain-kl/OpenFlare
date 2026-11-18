# 后端与边缘组件目录包结构重构

## 动机

在一次面向 Go 后端的全局结构审查后，针对此前存在的职责跨层、入口分裂与过度细分包等问题完成收敛：

1. **统一组件入口**：Server 主入口原位于仓库根目录 `main.go` + `internal/cmd/`，与边缘组件的 `cmd/{agent,relay,flared}/` 不对齐。
2. **共享域与边缘运行时解耦**：OpenFlare 专用的 `protocol`、`render/openresty`、`pagesarchive` 原放在通用 `pkg/`；边缘端侧 `agent`、`flared`、`relay`、`edge` 原混在 `internal/apps/` 控制面业务目录中。
3. **ClickHouse / logstore 合一**：`internal/repository/analytics/` 已完全被 `internal/repository/logstore/` 包装，保留两套独立仓储与重复转换/工具函数增加维护面。
4. **消除空壳桥接与过度拆分**：`internal/apps/upload/` 拆为 8 个子包再靠 `exports.go` 转发；`internal/router/v1/openflare/` 为每个域单列小文件；`agent`/`flared`/`relay` 及 `apiutil` 存在冗余转发别名；`internal/shared/constants.go` 仅含单个常量；`routeidentity`、`zone/legacy_import.go`、`infra/task/handlers/register.go` 存在逆向/跨层放置。

## 核心调整

- **入口收口 (`cmd/`)**：
  - `main.go` → `cmd/server/main.go`；`internal/cmd/` → `cmd/server/cli/`（`package cli`）。
  - `internal/buildinfo/` → `internal/platform/buildinfo/`，同步 `Makefile`、Dockerfiles、`scripts/swagger.sh`（`-d cmd/server,internal -g main.go`）与 CI 发布流。
- **共享域与运行时边界 (`internal/openflare/` & `internal/edge/`)**：
  - `pkg/protocol`、`pkg/render/openresty`、`pkg/pagesarchive`、原 `routeidentity` 迁入 `internal/openflare/{protocol,openresty,pagesarchive,routeidentity}`。
  - `internal/apps/{edge,agent,flared,relay}` 迁入 `internal/edge/{common,agent,flared,relay}`；清理薄转发子包（如端侧 `updater`/`wsclient`/`logging`/`protocol`/`geoipdata`），直接使用 `internal/edge/common/*` 与 `internal/openflare/protocol`。
- **仓储与分析库收口 (`internal/repository/logstore/`)**：
  - 将原 `repository/analytics` 的 ClickHouse 实现迁移为 `internal/repository/logstore/ch_*.go` 并移除 `internal/repository/analytics/`；补齐 `ObservabilityStore.ListLatestMetricSnapshots` 并用泛型查询助手 `listCHNodeObservability` 消除重复 SQL 模版。
  - `internal/apps/openflare/chwriter` 重命名为 `logwriter`；`admin/status/clickhouse.go` 改用注入式 `BatchWriterStatsProvider`。
- **业务与路由扁平化**：
  - `internal/apps/upload/{cache,filesrv,handler,ingest,shared,stats,storage,task,util}` 合并为单一 `package upload`；删除 `exports.go`。
  - `internal/apps/openflare/async_tasks*.go` → `tasks/register*.go`；移除 `tls/sensitive.go` 与各端控制面 `protocol_alias.go` 转发层；`admin/updater/export.go`、`shared/constants.go`、`router/{root,v1}/custom.go` 清理。
  - `apiutil/{bind,errors,routes}.go` → `internal/shared/httpx/`；OpenFlare V1 路由整合至 `internal/router/v1/{openflare_console,openflare_edge}.go`；跨模块集成测试迁入 `tests/integration/`；旧数据迁移逻辑移入 `internal/infra/persistence/migrator/zone_legacy_import.go`。
- **文档与技能同步**：同步 `AGENTS.md`、`.agents/skills/`、`docs/`、`docs/en/`、`docs/changelog/index.md` 及重构后生成的 Swagger 文档。

## 验证记录

- `make swagger`（`-d cmd/server,internal -g main.go`）：通过，210 条路由与原基线一致。
- `scripts/update_go_license.sh --check`：除工作区既有的未跟踪目录 `./SafeLine-main/` 外，仓库源码无缺失 SPDX 许可头。
- `make format`、`make code-check`（`golangci-lint run` 0 issues、PoW 资源一致性、i18n 4780 keys / 3 locales、runtime language check、TypeScript 与 ESLint）：全部通过。
- `make build-backend build-agent build-relay build-flared`：四个二进制全部构建成功。
- `go test -count=1 ./...`：全量非缓存单元测试与集成测试全部通过。
