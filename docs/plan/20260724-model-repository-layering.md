# model / repository 分层治理

说明：将 `internal/model` 收敛为无 IO 实体层，`internal/repository` 作为唯一持久化入口。

---

## 1. 目标与背景 (Goal & Context)

* **需求背景**：已提交的 AGENTS 曾允许 model 直接使用 GORM，导致 OpenFlare 业务 CRUD 与平台 repository 双轨并存。工作区目标分层与代码不一致，接手成本高。
* **开发范围 (Scope)**：
  * 固化规范：`model` 仅实体 / DTO / 无 IO 规则；`repository` 唯一持久化入口。
  * 将 `internal/model` 中现有 `db.DB` / Redis / ClickHouse store 适配迁入 `internal/repository`。
  * 全量更新 call site：`model.Get/List/Create…` → `repository.…`。
  * 编译通过 + `make format` + 相关单测。
* **Out of Scope**：不改表结构、不改 API 契约、不做业务行为变更；不强制一次重写所有测试风格。

## 2. 设计与决策

* **分层**：
  * `apps → repository → model`
  * `repository → infra/persistence`（及 `repository/analytics`）
  * **禁止** `model → repository`、**禁止** model 内 `db.DB` / Redis / ClickHouse
* **迁移策略**：按文件拆分 package-level IO 函数至同名 `repository` 文件；类型与纯函数留在 model；store 适配整文件迁入 repository。
* **命名**：repository 函数保持原导出名，降低 call site 改动面。

## 3. 具体修改文件清单

### 规范

* #### [MODIFY] `AGENTS.md`
* #### [MODIFY] `docs/design/index.md`
* #### [MODIFY] `docs/plan/index.md`（登记本计划）

### 后端

* #### [MODIFY] `internal/model/*.go`（剥离 IO）
* #### [NEW/MODIFY] `internal/repository/*.go`（承接 CRUD / store）
* #### [MODIFY] `internal/apps/**`、`internal/infra/task/**` 等 call site

## 4. 验证计划

* `go test ./internal/model/... ./internal/repository/...`
* 关键包 `go build ./...`
* `make format` / `make code-check`（在可接受时间内）
