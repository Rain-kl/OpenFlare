---
name: "new-api"
description: "Wavelet 项目专用：当新增或修改自定义业务 API、新增业务路由、新增 service 层核心逻辑时必须使用。本技能指导包职责划分、推荐文件结构、路由解耦、Swagger 文档生成与质量门禁验证。"
---

# 新增业务 API 开发与路由注册规范

本技能是 Wavelet 项目接口开发与路由注册的唯一指导规范。在开发任何新接口前，请严格按照本指南进行架构决策与路由注册。

---

## 核心路由准则与防线 (Routing Governance & Guardrails)

Wavelet 后端路由采用了**严格的框架层与业务层隔离机制**。请牢记以下开发原则：

### 插件目录标准结构 (`backend/openflare/plugins/<name>/` 或 `backend/plugins/domain/<name>/`)

所有标准插件与下游定制插件，**统一以 `backend/downstream/plugins/custom_example` 为基准模板**，严格采用物理子包隔离的分层架构：

```text
backend/openflare/plugins/<name>/ (或 backend/plugins/domain/<name>/)
├── plugin.go           # 插件根入口：实现 core.Plugin，装配各子包并向 Cordis 注册
│
├── consts/             # package consts：常量、配置键名与错误码定义
│   └── consts.go
│
├── controller/         # package controller：HTTP 控制器与路由声明 (参数绑定、会话获取、信封响应)
│   └── hello/          # 业务分组/实体子包
│       └── hello.go    # 接口处理 Handler（直接以业务命名，禁止 controller_hello.go）
│
├── service/            # package service：业务逻辑层（用例编排、事务控制、事件发布）
│   └── order.go        # 订单业务用例实现（纯 Go 逻辑，禁止依赖 *gin.Context）
│
├── dao/                # package dao：数据访问持久化层 DAL (GORM CRUD、SQL 转义防注入)
│   └── order.go        # 订单数据访问实现（直接以业务命名，禁止 dao_order.go）
│
├── model/              # package model：纯数据实体与 DTO（无外部依赖）
│   ├── entity/         # 数据库映射实体 (TableName() 带插件专属前缀)
│   │   └── order.go
│   └── do/             # 请求 Request DTO 与响应 Response DTO、领域对象
│       └── order.go
│
└── migrations/         # 专属嵌入式 Goose SQL 双方言迁移脚本 (//go:embed)
    ├── postgres/       # PostgreSQL 迁移脚本
    └── sqlite/         # SQLite 迁移脚本
```
> ⚠️ **严禁**：严禁在根目录平铺 `handlers_*.go`、`service_*.go`、`dao_*.go` 等前缀文件，子包内文件直接按业务实体命名。严格约束 `controller -> service -> dao -> model` 单向依赖。

---

## 核心开发步骤 (Step-by-Step Flow)

### 步骤 1：数据库定义与迁移
如果自定义功能涉及新表或字段，请参考 [database-migration](../database-migration/SKILL.md) 技能，在 `internal/infra/persistence/migrator/goose/` 目录下编写迁移文件，在 `internal/model/` 中定义 GORM 实体（无 CRUD / 无 DB 访问），并在 `internal/repository/` 中实现数据访问（**repository 为唯一持久化入口**）。

### 步骤 2：在模块内实现业务逻辑 (`logics.go` / `service.go`)
业务逻辑逻辑应当实现于 `internal/apps/custom/` 目录下：
- **优先使用纯函数（`logics.go`）**：定义接收 `context.Context` 且不依赖 `*gin.Context` 的函数，易于单元测试与 Worker 复用。参考 `internal/apps/user/logics.go`。
- **有状态服务（`service.go`）**：若需注入依赖（如 DB 连接、外部客户端等），可定义 Service 结构体和构造函数。
- **跨模块副作用（推送、任务监听等）**：核心业务代码通过 `internal/listener` 发射域事件，禁止直接 `import` push 模块；装配在 `internal/platform/bootstrap` 完成（参见 `push-notification` skill）。

### 步骤 3：编写 HTTP Handler (`routers.go`)
在 `internal/apps/custom/routers.go` 中编写 Handler：
- 负责请求参数绑定与校验（使用 `ShouldBindJSON`/`ShouldBindQuery`）。
- 负责提取 Session / 用户身份。
- 调用业务逻辑层，并使用 `OpenFlare/internal/shared/response` 统一返回响应：
  - 成功时返回：`response.OK(data)` 或 `response.OKNil()`
  - 失败时返回：`response.Err(msg)`
- 编写规范的 Swagger 注释。

### 步骤 4：在自定义包中注册路由并委派
根据 **路由归属判定表**，在 [root/custom.go](file:///Users/ryan/DEV/Go/Wavelet/internal/router/root/custom.go) 或 [v1/custom.go](file:///Users/ryan/DEV/Go/Wavelet/internal/router/v1/custom.go) 中编写注册代码，将路由路径绑定到步骤 3 中编写的 Handler。

---

## 质量验证门禁 (Quality Gates)

每次新增或修改接口后，必须运行并验证以下各项：
1. **自动授权许可**：`make license`（新增 Go 文件时自动添加许可头）
2. **重新生成 Swagger 文档**：`make swagger`（若有 Swagger 注释修改）
3. **静态代码及风格检查**：`make code-check`（确保通过 golangci-lint 和前端 TS 检查）
4. **自动化单元测试**：`go test ./...`（确保所有测试 100% 通过）
