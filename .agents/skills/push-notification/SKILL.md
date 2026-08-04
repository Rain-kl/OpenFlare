---
name: "push-notification"
description: "Wavelet 项目专用：当需要开发或接入新的系统通知推送事件、修改消息推送底层设计、调用统一触发器投递消息、或开发带消息推送功能的业务功能时必须使用。本技能指导元数据声明、触发流程、解耦防线和动态同步机制。"
---

# 新增消息推送与通知事件开发规范

本技能涵盖 Wavelet 的系统通知推送开发规范。开始开发前先阅读仓库根目录 [AGENTS.md](file:///Users/ryan/DEV/Go/Wavelet/AGENTS.md)，遵守项目级核心规则。

---

## 消息推送架构设计 (Architecture)

Wavelet 的消息推送机制采用了**元数据驱动 + 统一触发器 + 异步任务派发**的解耦设计，其分层及职责划分如下：

| 目录/包名 | 职责定位 | 包含内容与设计细节 |
| :--- | :--- | :--- |
| **`pkg/push/`** | 推送基础设施层 | 静态定义、不依赖系统数据库和任何框架。定义了统一接口 `Pusher`、单例 `PusherPool` 和多实现（Lark, Webhook, Email 等），提供配置验证及发送功能。 |
| **`internal/apps/admin/push/`** | 通知服务与后台任务层 | 包含以下核心文件：<br>1. [events.go](file:///Users/ryan/DEV/Go/Wavelet/internal/apps/admin/push/events.go)：定义通知事件的结构模型（`NotificationMessage`, `EventMetadata`）、内置事件的动态注册中心（`BuiltInEvents` 及 `RegisterBuiltInEvent` 函数）以及统一触发器类 `EventTrigger`（包括其底层的派发引擎逻辑）。<br>2. [tasks.go](file:///Users/ryan/DEV/Go/Wavelet/internal/apps/admin/push/tasks.go)：定义 Asynq 后台异步发送任务、处理器 `PushHandler` 及其校验逻辑，并记录推送历史审计。<br>3. [routers.go](file:///Users/ryan/DEV/Go/Wavelet/internal/apps/admin/push/routers.go)：管理端接口，负责获取事件配置列表和更新配置。 |
| **`internal/apps/admin/push/custom_events/`** | 自定义通知事件包 | 事件元数据定义与 push 侧处理逻辑；**一个 Go 文件代表一个事件**。在 [register.go](file:///Users/ryan/DEV/Go/Wavelet/internal/apps/admin/push/custom_events/register.go) 统一装配，禁止 `init()` 副作用。 |
| **`internal/listener/`** | 域事件分发层 | 核心域发射事件（如 `EmitAdminLoggedIn`），push 在 bootstrap 阶段通过 `OnAdminLoggedIn` 订阅，避免 auth/user 直接依赖 push。 |
| **`internal/platform/bootstrap/`** | 应用装配根 | `RegisterPushDomainEvents()` 调用 `custom_events.Register()`；`Init` 中执行 `SyncEvents` 将内置事件元数据同步到数据库。 |
| **数据库审计表** | 状态与历史审计 | `w_push_events` 存放每个通知事件的启用状态、启用渠道、发送目标和自定义渲染模板。<br>`w_push_histories` 存放消息发送记录用于审计。 |

---

## 核心开发步骤 (Step-by-Step Flow)

如果某个新业务（如“新用户注册”或“订单创建”）需要带有消息推送功能，请严格按照以下步骤开发：

### 步骤 1：在 `custom_events/` 中声明事件元数据与处理函数
在 `internal/apps/admin/push/custom_events/` 下新建一个 Go 文件（如 `user_registered.go`），声明 `EventMetadata` 和 push 侧处理函数（组装 body 并调用 `DefaultTrigger.Trigger`）。

```go
package custom_events

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/listener"
)

var NewUserRegistered = push.EventMetadata{
	Key:  "user_registered",
	Name: "新用户注册提醒",
	DefaultTemplate: push.NotificationMessage{
		Title:   "新用户注册通知",
		Content: "新用户 {{user.username}} (邮箱: {{user.email}}) 于 {{time}} 成功注册。",
		Level:   "INFO",
	},
	Description: "当系统有新用户注册成功时，向管理员或指定目标发送通知",
}

func handleUserRegistered(ctx context.Context, event listener.UserRegistered) {
	if event.User == nil {
		return
	}
	body := map[string]any{
		"user": event.User,
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	push.DefaultTrigger.Trigger(ctx, NewUserRegistered, body)
}
```

> `EventTrigger.Trigger` 已内置异步 Goroutine 与 `context.WithoutCancel`；处理函数内直接调用即可，无需外层 `go func()`。

### 步骤 2：在 `listener/` 定义域事件并在 `register.go` 装配
1. 在 `internal/listener/` 新增域事件类型、`Emit*` 与 `On*` 注册函数（参考 [admin_login.go](file:///Users/ryan/DEV/Go/Wavelet/internal/listener/admin_login.go)）。
2. 在 [register.go](file:///Users/ryan/DEV/Go/Wavelet/internal/apps/admin/push/custom_events/register.go) 中注册元数据并订阅域事件：

```go
func Register() {
	push.RegisterBuiltInEvent(NewUserRegistered)
	listener.OnUserRegistered(handleUserRegistered)
}
```

**禁止**在 `custom_events` 或 `router` 中使用 `init()` 注册；**禁止**在 `router.go` 空白导入 `custom_events`。

### 步骤 3：在业务代码中发射域事件（不 import push）
在业务逻辑完成处（如 `internal/apps/user/routers.go`）仅 import `internal/listener` 并发射事件：

```go
import "github.com/Rain-kl/Wavelet/internal/listener"

func Register(c *gin.Context) {
	// ... 注册成功逻辑 ...
	listener.EmitUserRegistered(ctx, user)
}
```

### 步骤 4：在 bootstrap / cmd 入口显式装配
新增事件后，确保 `custom_events.Register()` 已被 `bootstrap.RegisterPushDomainEvents()` 调用，且 API/`all` 进程在 `bootstrap.Init` 之前完成注册：

| 进程 | cmd 入口调用 |
| :--- | :--- |
| `api` | `bootstrap.RegisterAPI()` → `bootstrap.Init(ctx, Options{API: true})` |
| `all` | `bootstrap.RegisterAll()` → `bootstrap.Init(ctx, Options{API: true})` |
| `worker` / `scheduler` | 不注册 push 域事件；仅 `bootstrap.Init` + 各自 `RegisterWorker`/`RegisterScheduler` |

`Init` 中的 `SyncEvents` 会将 `user_registered` 元数据同步到 `w_push_events`，管理员即可在前端配置推送渠道。

### 步骤 5：编写集成测试
在 `custom_events/` 或 `listener/` 包内添加测试，验证 `Emit*` → handler → `DefaultTrigger.Trigger` 全链路。测试 setup 须显式调用 `custom_events.Register()`（或 `bootstrap.RegisterPushDomainEvents()`）和 `push.SyncEvents`，参考 [admin_login_test.go](file:///Users/ryan/DEV/Go/Wavelet/internal/apps/admin/push/custom_events/admin_login_test.go)。

---

## 模板渲染与支持的系统变量 (Template Rendering & Variables)

消息的 `title`、`content` 以及 `ext` 字段中的字符串值都支持变量占位符替换，采用双花括号形式 `{{variable}}`。

### 1. 通用事件参数 (Common Variables)
在 Wavelet 系统中，`user` 是一个通用的、必传的事件参数。如果在触发通知事件时未提供 `user`（或为 `nil`），底层 `EventTrigger` 会自动注入一个系统的虚拟用户（ID 为 999，昵称为“系统”）。因此，以下变量是所有通知事件均支持的通用渲染参数：

- `{{time}}`：事件发生/触发的具体时间（格式：`2006-01-02 15:04:05`）
- `{{user.id}}`：触发用户/系统用户的 ID
- `{{user.username}}`：触发用户/系统用户的用户名
- `{{user.nickname}}`：触发用户/系统用户的昵称
- `{{user.email}}`：触发用户/系统用户的电子邮箱
- `{{user.phone}}`：触发用户/系统用户的手机号
- `{{user.bio}}`：触发用户/系统用户的个人简介
- `{{user.gender}}`：触发用户/系统用户的性别
- `{{user.location}}`：触发用户/系统用户的所在地
- `{{user.website}}`：触发用户/系统用户的个人网站

*(注：系统中的任何自定义事件，若传入了对应的复杂结构体，其结构体 JSON 字段均可通过扁平化点路径方式直接在模板中进行引用。)*

### 2. 特定事件携带的业务变量 (Event Specific Variables)
除了通用的 `user` 和 `time` 外，特定事件在触发时还可以携带额外的上下文参数：

- **管理员登录提醒 (`admin_login`)**
  - `{{ip}}`：管理员登录来源的客户端 IP
  - `{{time}}`：管理员登录成功时间

### 3. 自定义消息通道的请求体变量说明 (Custom Channel JSON Variables)
在配置“自定义消息通道”时，其请求体 (JSON Schema) 支持以 `$` 开头的变量替换。支持的替换变量如下：

```json
{
  "title": "$title",
  "description": "$description",
  "content": "$content",
  "url": "$url",
  "to": "$to"
}
```

- `$title`：通知的标题（如：“管理员登录提醒”）
- `$description`：当前通知事件的描述
- `$content`：通知的具体渲染后正文内容
- `$url`：附加的操作或详情链接（若有）
- `$to`：当前派发的推送目标（如邮箱、ID 或 Chat ID，即 resolved target）

---

## 严格遵循事项与防线 (Guardrails)

### 1. 禁止绕过统一触发器 (Always Use EventTrigger)
- 所有推送请求必须经过 `EventTrigger.Trigger`，以确保进行“事件是否启用”、“目标渠道过滤”、“全局推送配置读取”及“发送日志审计”等流程。

### 2. 禁止业务模块直接依赖 push (Decouple via listener)
- `oauth`、`user` 等核心域 **不得** `import` `internal/apps/admin/push` 或 `custom_events`。
- 跨模块通知必须通过 `internal/listener` 发射域事件；push 在 `custom_events.Register()` 中订阅。

### 3. 禁止 init() 与 router 副作用注册 (Explicit Bootstrap)
- 不得在 `init()` 中调用 `RegisterBuiltInEvent` 或订阅 listener。
- 不得在 `router.go` 空白导入 `custom_events` 触发注册。
- 统一在 `internal/platform/bootstrap` + `internal/cmd` 入口显式装配。
