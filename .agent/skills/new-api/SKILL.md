---
name: "new-api"
description: "Wavelet 项目专用：当新增或修改自定义业务 API、新增业务路由、新增 service 层核心逻辑时必须使用。本技能指导包职责划分、推荐文件结构、路由解耦、Swagger 文档生成与质量门禁验证。"
---

# 新增业务 API 开发与路由注册规范

本技能是 Wavelet 项目接口开发与路由注册的唯一指导规范。在开发任何新接口前，请严格按照本指南进行架构决策与路由注册。

---

## 核心路由准则与防线 (Routing Governance & Guardrails)

Wavelet 后端路由采用了**严格的框架层与业务层隔离机制**。请牢记以下开发原则：

1. **禁止修改框架级路由文件**：
   - 以下文件属于系统框架/平台级接口，**禁止为了添加自定义业务接口而进行任何修改**：
     - `internal/router/router.go`（核心入口委派）
     - `internal/router/root/default.go`（公开文件服务、robots.txt、Swagger 及 /api/health 路由）
     - `internal/router/root/frontend.go`（前端静态服务）
     - `internal/router/v1/v1.go`（V1 分发层协调器）
     - `internal/router/v1/admin.go`（框架管理员端管理接口）
     - `internal/router/v1/user.go`（框架普通用户端基础接口、OAuth及公开接口）
2. **仅允许在 `custom.go` 中注册业务接口**：
   - 所有的自定义/业务相关接口注册，有且仅有以下两个合法的承载点：
     - [internal/router/root/custom.go](file:///Users/ryan/DEV/Go/Wavelet/internal/router/root/custom.go)（用于挂载到根路径的特殊业务接口）
     - [internal/router/v1/custom.go](file:///Users/ryan/DEV/Go/Wavelet/internal/router/v1/custom.go)（用于挂载在 API V1 下的标准自定义业务接口）

---

## 路由归属判定表 (Where should I register my new API?)

根据接口的**访问路径特征**和**访问身份/限制条件**，决定将新开发的 API 挂载至何处：

| 目标 API 路径特征 | 访问身份/条件限制 | 对应的路由注册入口 | 是否允许修改 |
| :--- | :--- | :--- | :--- |
| **`/my-custom-path`** (挂载在根路径下的特殊业务接口) | 自定义控制 | `root/custom.go` 中的 `RegisterCustomRootRoutes` | **允许修改 (业务自定义入口)** |
| **`/api/v1/custom/...`** (API v1 下的定制业务接口) | 自定义控制 | `v1/custom.go` 中的 `RegisterCustomRoutes` | **允许修改 (业务自定义入口)** |
| **`/api/v1/admin/...`** (系统管理员管理端接口) | 需要管理员登录 (`admin.LoginAdminRequired()`) | `v1/admin.go` | **禁止修改 (仅限系统框架路由)** |
| **`/api/v1/user/...`** (框架普通用户基础接口) | 需要普通用户登录 (`oauth.LoginRequired()`) | `v1/user.go` | **禁止修改 (仅限系统框架路由)** |
| **`/api/v1/public/...`** (Captcha、Config 等系统公开接口) | 所有人 (无条件 / 公开) | `v1/user.go` | **禁止修改 (仅限系统框架路由)** |
| **`GET /f/:id`**, **`GET /robots.txt`**, **`GET /api/health`** (系统级默认及公开接口) | 所有人 (无条件 / 公开) | `root/default.go` | **禁止修改 (仅限系统框架路由)** |

---

## 两个自定义路由包的用法与区别 (Root Custom vs V1 Custom)

### 1. 根路径自定义包：`root/custom.go`

* **适用场景**：适用于需要**直接挂载在主域名根路径下**的特殊自定义业务接口（如第三方 Webhook 回调、特定的短链接重定向、外部数据接口等，不需要 `/api/v1` 前缀）。
* **用法示例**：
  在 [root/custom.go](file:///Users/ryan/DEV/Go/Wavelet/internal/router/root/custom.go) 中实现：
  ```go
  package root

  import (
  	"github.com/Rain-kl/Wavelet/internal/apps/custom"
  	"github.com/gin-gonic/gin"
  )

  // RegisterCustomRootRoutes registers custom business routes that belong to the root path.
  func RegisterCustomRootRoutes(r *gin.Engine) {
  	// 挂载到根路径下，如 GET /my-custom-webhook
  	r.GET("/my-custom-webhook", custom.HandleRootWebhook)
  }
  ```
  *(注：该函数已由 `root.go` 自动加载，你无需修改任何其他核心文件。)*

### 2. V1 API 自定义包：`v1/custom.go`

* **适用场景**：适用于普通的**自定义业务 API**，需要规范挂载在标准 API V1 路径下（即自动带有 `/api/v1/custom/...` 前缀，可选择性配置用户/管理员登录中间件）。
* **用法示例**：
  在 [v1/custom.go](file:///Users/ryan/DEV/Go/Wavelet/internal/router/v1/custom.go) 中实现：
  ```go
  package v1

  import (
  	"github.com/Rain-kl/Wavelet/internal/apps/custom"
  	"github.com/gin-gonic/gin"
  )

  // RegisterCustomRoutes registers standard custom API routes under /api/v1.
  func RegisterCustomRoutes(apiV1Router *gin.RouterGroup) {
  	customRouter := apiV1Router.Group("/custom")
  	{
  		// 挂载到 /api/v1/custom 下，例如：POST /api/v1/custom/action
  		customRouter.POST("/action", custom.DoActionHandler)
  	}
  }
  ```
  *(注：该函数已由 `v1/v1.go` 自动加载，你无需修改任何其他核心文件。)*

---

## 建议创建/修改的文件结构 (Recommended Directory Structure)

当新增一套定制的业务接口（例如名为 `custom` 的业务模块）时，建议采用以下标准文件结构：

```text
internal/
├── router/
│   ├── root/
│   │   └── custom.go           # [修改] 若为根路径 API，在此处注册，将路由委派给 apps/custom
│   └── v1/
│       └── custom.go           # [修改] 若为 v1 API，在此处注册，将路由委派给 apps/custom
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
