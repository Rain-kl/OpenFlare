---
name: "new-api"
description: "Wavelet 项目专用：当新增或修改自定义业务 API、新增业务路由、新增 service 层核心逻辑时必须使用。本技能指导包职责划分、推荐文件结构、路由解耦、Swagger 文档生成与质量门禁验证。"
---

# 新增业务 API 开发与路由注册规范

本技能是 Wavelet 项目接口开发与路由注册的唯一指导规范。在开发任何新接口前，请严格按照本指南进行架构决策与路由注册。

---

## 核心路由准则与防线 (Routing Governance & Guardrails)

Wavelet 后端路由采用了**严格的框架层与业务层隔离机制**。请牢记以下开发原则：

1. **路由器与业务解耦**：
   - `internal/router/router.go` 只负责顶层引擎配置与委派，**禁止直接挂载业务 Handler**：
     - `internal/router/root/root.go`、`default.go`（公开文件服务、robots.txt、Swagger）、`frontend.go`（前端静态服务）
     - `internal/router/v1/v1.go`（V1 分发层协调器）
     - `internal/router/v1/admin.go`（系统管理员端管理接口）
     - `internal/router/v1/user.go`（普通用户端基础接口、OAuth 及公开接口）
     - `internal/router/v1/openflare_console.go`（OpenFlare 控制台管理接口）
     - `internal/router/v1/openflare_edge.go`（Agent / Relay / Tunnel 边缘节点协议接口）
2. **新业务路由放置原则**：
   - 已有域接口扩充：在对应的领域路由文件（如 `internal/router/v1/admin.go`、`user.go`、`openflare_console.go`）中挂载，**不要**堆砌进 `router.go` 或 `v1.go`。
   - 全新独立域 / 根路径特殊路由：在 `internal/router/v1/<domain>.go`（或 `internal/router/root/<domain>.go`）新建独立注册文件与 `register<Domain>Routes(...)` 函数，并在 `v1.go`（或 `root.go`）中调用。

---

## 路由归属判定表 (Where should I register my new API?)

根据接口的**访问路径特征**和**访问身份/限制条件**，决定将新开发的 API 挂载至何处：

| 目标 API 路径特征 | 访问身份/条件限制 | 对应的路由注册入口 |
| :--- | :--- | :--- |
| **`/my-custom-path`** (挂载在根路径下的特殊业务接口) | 按需配置中间件 | `internal/router/root/<domain>.go`，由 `root/root.go` 调用 |
| **`/api/v1/<domain>/...`** (API v1 下的独立新业务域) | 按需配置中间件 | `internal/router/v1/<domain>.go`，由 `v1/v1.go` 调用 |
| **`/api/v1/d/...`** / **`/api/v1/agent/...`** (OpenFlare 控制面/节点协议接口) | 管理员或节点 Token 鉴权 | `v1/openflare_console.go` / `v1/openflare_edge.go` |
| **`/api/v1/admin/...`** (系统管理员管理端接口) | 需要管理员登录 (`admin.LoginAdminRequired()`) | `v1/admin.go` |
| **`/api/v1/user/...`** (普通用户基础接口) | 需要普通用户登录 (`oauth.LoginRequired()`) | `v1/user.go` |
| **`/api/v1/public/...`** (Captcha、Config 等系统公开接口) | 所有人 (无条件 / 公开) | `v1/user.go` |
| **`GET /f/:id`**, **`GET /robots.txt`**, **`GET /swagger/*any`** (系统级根路径接口) | 所有人 (无条件 / 公开) | `root/default.go` |

---

## 两个路由分层示例 (Root vs V1)

### 1. 根路径扩展：`internal/router/root/<domain>.go`

* **适用场景**：适用于需要**直接挂载在主域名根路径下**的特殊业务接口（如第三方 Webhook 回调、短链接重定向等，不需要 `/api/v1` 前缀）。
* **用法示例**：
  在 `internal/router/root/<domain>.go` 中实现，并在 `internal/router/root/root.go` 的 `RegisterRootRoutes` 中调用：
  ```go
  package root

  import (
  	"github.com/Rain-kl/Wavelet/internal/apps/custom"
  	"github.com/gin-gonic/gin"
  )

  func registerCustomRootRoutes(r *gin.Engine) {
  	// 挂载到根路径下，如 GET /my-custom-webhook
  	r.GET("/my-custom-webhook", custom.HandleRootWebhook)
  }
  ```

### 2. V1 API 扩展：`internal/router/v1/<domain>.go`

* **适用场景**：适用于标准的**业务 API**，挂载在 `/api/v1/...` 前缀下，可按需挂载用户/管理员中间件。
* **用法示例**：
  在 `internal/router/v1/<domain>.go` 中实现，并在 `internal/router/v1/v1.go` 的 `RegisterV1Routes` 中调用：
  ```go
  package v1

  import (
  	"github.com/Rain-kl/Wavelet/internal/apps/custom"
  	"github.com/gin-gonic/gin"
  )

  func registerCustomRoutes(apiV1Router *gin.RouterGroup) {
  	customRouter := apiV1Router.Group("/custom")
  	{
  		// 挂载到 /api/v1/custom 下，例如：POST /api/v1/custom/action
  		customRouter.POST("/action", custom.DoActionHandler)
  	}
  }
  ```

---

## 建议创建/修改的文件结构 (Recommended Directory Structure)

当新增一套定制的业务接口（例如名为 `custom` 的业务模块）时，建议采用以下标准文件结构：

```text
internal/
├── router/
│   ├── root/
│   │   ├── root.go             # [按需修改] 委派调用根路径模块注册函数
│   │   └── <domain>.go         # [新建/修改] 根路径业务路由注册
│   └── v1/
│       ├── v1.go               # [按需修改] 委派调用 V1 模块注册函数
│       └── <domain>.go         # [新建/修改] V1 业务路由注册
└── apps/
    └── custom/
        ├── routers.go          # [新建] HTTP Handlers (Gin)，负责参数绑定、校验与响应
        ├── logics.go           # [新建] 业务逻辑层：承载模块内闭环的纯 Go 业务逻辑，不依赖 gin.Context
        └── errs.go             # [新建] 存放模块特有的业务错误常量定义（可选）
```

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
- 调用业务逻辑层，并使用 `github.com/Rain-kl/Wavelet/internal/shared/response` 统一返回响应：
  - 成功时返回：`c.JSON(http.StatusOK, response.OK(data))` 或 `response.OKNil()`
  - 失败时使用：`response.AbortBadRequest(c, ...)`、`response.AbortInternal(c, ...)` 等由 `ErrorHandlerMiddleware` 统一输出
- 编写规范的 Swagger 注释。

### 步骤 4：在模块路由注册文件中挂载路由
根据 **路由归属判定表**，在对应的 `internal/router/root/<domain>.go` 或 `internal/router/v1/<domain>.go` 中编写注册代码，将路由路径绑定到步骤 3 中编写的 Handler。

---

## 质量验证门禁 (Quality Gates)

每次新增或修改接口后，必须运行并验证以下各项：
1. **自动授权许可**：`make license`（新增 Go 文件时自动添加许可头）
2. **重新生成 Swagger 文档**：`make swagger`（若有 Swagger 注释修改）
3. **静态代码及风格检查**：`make code-check`（确保通过 golangci-lint 和前端 TS 检查）
4. **自动化单元测试**：`go test ./...`（确保所有测试 100% 通过）
