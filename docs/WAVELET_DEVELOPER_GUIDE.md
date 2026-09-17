# Wavelet Cordis 插件化架构实战开发指南与标准规范

- **文档类型**: 下游开发者手册 / 架构实战指南 (Cookbook & Architecture Reference)
- **目标受众**: 官方插件开发者、下游业务二开工程师、架构师
- **版本**: v1.0.0 (2026-08-27)

---

# 目录
- [第一部分：下游项目实战开发指南与 22 个高频开发场景解答](#第一部分下游项目实战开发指南与-22-个高频开发场景解答)
  - [场景 1：插件必须要实现哪些方法与契约？](#场景-1插件必须要实现哪些方法与契约)
  - [场景 2：插件间如何进行单向服务调用？](#场景-2插件间如何进行单向服务调用)
  - [场景 3：插件间存在双向/循环调用时如何解决（杜绝 import cycle）？](#场景-3插件间存在双向循环调用时如何解决杜绝-import-cycle)
  - [场景 4：如何开发并注册一个 HTTP API 接口？如何添加路由中间件？](#场景-4如何开发并注册一个-http-api-接口如何添加路由中间件)
  - [场景 5：如何获取当前登录用户信息？](#场景-5如何获取当前登录用户信息)
  - [场景 6：如何开发并注册一个 Asynq 异步 Worker 任务？](#场景-6如何开发并注册一个-asynq-异步-worker-任务)
  - [场景 7：如何开发并注册一个 Cron 定时任务？](#场景-7如何开发并注册一个-cron-定时任务)
  - [场景 8：数据库表结构如何声明？ORM 模型规范是什么？](#场景-8数据库表结构如何声明orm-模型规范是什么)
  - [场景 9：数据库如何做独立迁移？Goose SQL 怎么组织？](#场景-9数据库如何做独立迁移goose-sql-怎么组织)
  - [场景 10：如果有多个业务插件需要读写同一张表怎么办？](#场景-10如果有多个业务插件需要读写同一张表怎么办)
  - [场景 11：如果跨插件操作多张表，如何确保事务一致性？](#场景-11如果跨插件操作多张表如何确保事务一致性)
  - [场景 12：如何发布和订阅领域事件 (EventBus)？](#场景-12如何发布和订阅领域事件-eventbus)
  - [场景 13：如何向系统注册插件自定义配置（config.yaml 与管理台热加载设置）？](#场景-13如何向系统注册插件自定义配置configyaml-与管理台热加载设置)
  - [场景 14：如何使用多层缓存（RAM L1 + Redis L2 + PubSub 同步）？](#场景-14如何使用多层缓存ram-l1--redis-l2--pubsub-同步)
  - [场景 15：如何使用分布式锁 (DistLock) 防止并发超卖与重复消费？](#场景-15如何使用分布式锁-distlock-防止并发超卖与重复消费)
  - [场景 16：如何向管理后台动态注册监控数据与管理控制台？](#场景-16如何向管理后台动态注册监控数据与管理控制台)
  - [场景 17：插件如何实现健康检查探针与就绪检查 (Health Check)？](#场景-17插件如何实现健康检查探针与就绪检查-health-check)
  - [场景 18：插件如何扩展其他插件的能力（如新增一种 OAuth 登录提供商 / 新增消息推送渠道）？](#场景-18插件如何扩展其他插件的能力如新增一种-oauth-登录提供商--新增消息推送渠道)
  - [场景 19：插件如何编写单元测试与集成测试（Mock 上下文与依赖打桩）？](#场景-19插件如何编写单元测试与集成测试mock-上下文与依赖打桩)
  - [场景 20：以不同角色（api / worker / schedule / all）启动时，插件代码如何适配？](#场景-20以不同角色api--worker--schedule--all启动时插件代码如何适配)
  - [场景 21：当某个插件流量暴增需要独立拆分为微服务时，如何零成本平滑改造？](#场景-21当某个插件流量暴增需要独立拆分为微服务时如何零成本平滑改造)
  - [场景 22：插件如何安全处理文件上传与大文件摄取 (upload.Ingest)？](#场景-22插件如何安全处理文件上传与大文件摄取-uploadingest)
- [第二部分：整个项目的目录结构划分与包职责定义](#第二部分整个项目的目录结构划分与包职责定义)
- [第三部分：框架核心提供给插件调用的公用能力矩阵 (Context Capability Matrix)](#第三部分框架核心提供给插件调用的公用能力矩阵-context-capability-matrix)
- [第四部分：Cordis 插件分层开发规范与代码模板 (Plugin Layered Architecture & Code Templates)](#第四部分cordis-插件分层开发规范与代码模板-plugin-layered-architecture--code-templates)
  - [1. 统一标准分层架构与目录规范 (以 custom_example 为基准)](#1-统一标准分层架构与目录规范-以-custom_example-为基准)
  - [2. 核心分层代码模板与实现范例](#2-核心分层代码模板与实现范例)
  - [3. 各层核心职责边界与严格禁止防线 (Guardrails)](#3-各层核心职责边界与严格禁止防线-guardrails)

---

# 第一部分：下游项目实战开发指南与 22 个高频开发场景解答

### 场景 1：插件必须要实现哪些方法与契约？
每个插件必须实现 `core.Plugin` 接口，仅需提供两个核心方法：`Name()` 与 `Apply(ctx *core.Context)`。

```go
package myplugin

import "github.com/Rain-kl/Wavelet/core"

type Plugin struct{}

// 1. Name: 返回全局唯一的插件标识符（建议遵循命名空间规范，如 "biz.order"）
func (p *Plugin) Name() string {
    return "biz.order"
}

// 2. Apply: 核心装载入口，所有的路由注册、任务注册、服务提供与依赖消费均在此完成
func (p *Plugin) Apply(ctx *core.Context) error {
    // 在此编写装载逻辑
    return nil
}
```

---

### 场景 2：插件间如何进行单向服务调用？
**规则**：插件之间**禁止直接相互 import 具体实现包**。调用方仅面向 `core/contracts` 中的纯 Interface 编程，运行时通过 Context 解析。

```go
// 1. 插件 A (提供者 plugins/user) 将服务注入 Context
func (p *UserPlugin) Apply(ctx *core.Context) error {
    dbSvc, _ := core.Inject[contracts.DBService](ctx)
    userSvc := NewUserServiceImpl(dbSvc)
    core.Provide[contracts.UserService](ctx, userSvc)
    return nil
}

// 2. 插件 B (消费者 plugins/order) 声明依赖并调用
func (p *OrderPlugin) Apply(ctx *core.Context) error {
    return ctx.Using(func(userSvc contracts.UserService) {
        // userSvc 已由容器自动注入就绪
        v1 := ctx.Router().Group("/api/v1/orders")
        v1.POST("", func(c *gin.Context) {
            userInfo, err := userSvc.GetUserProfile(c.Request.Context(), "user_123")
            // 处理订单逻辑...
        })
    })
}
```

---

### 场景 3：插件间存在双向/循环调用时如何解决（杜绝 import cycle）？
**问题场景**：`auth` 登录成功后需要查 `user` 资料；`user` 重置密码后需要调 `auth` 吊销 session。若两个 package 互相 import，Go 编译器会报 `import cycle not allowed`。

**Cordis 解法**：
1. 接口均定义在 `core/contracts`，双方只依赖 `core/contracts`。
2. 运行时采用 **延迟注入 (Lazy Resolution / Inject)** 或 **事件解耦 (EventBus)**：

```go
// plugins/auth/service.go
func (s *AuthServiceImpl) OnLoginSuccess(c context.Context, uid string) {
    // 延迟注入 UserService，不发生 package 级循环导入
    userSvc, err := core.Inject[contracts.UserService](s.ctx)
    if err == nil {
        userSvc.UpdateLastLoginTime(c, uid)
    }
}
```
*更加推荐的方式是发射领域事件*（见场景 12），由 `user` 插件自愿监听，彻底消除相互调用的硬依赖。

---

### 场景 4：如何开发并注册一个 HTTP API 接口？如何添加路由中间件与白名单？
插件通过 `ctx.Router()` 声明路由。微内核支持标准 Gin 路由组、中间件挂载与免鉴权白名单机制：

```go
func (p *OrderPlugin) Apply(ctx *core.Context) error {
    // 1. 如果插件包含无需登录的公开接口，主动注册到 Router 白名单（支持精确路径与通配符如 /api/v1/public/*）
    ctx.Router().RegisterWhitelist(
        "/api/v1/orders/public-status",
        "/api/v1/orders/callback/*",
    )

    // 2. 获取全局或 auth 插件提供的鉴权中间件
    authSvc, _ := core.Inject[contracts.AuthService](ctx)

    // 3. 创建带版本前缀和鉴权中间件的路由组
    group := ctx.Router().Group("/api/v1/orders", authSvc.RequireAuthMiddleware())
    
    // 4. 注册 Handler
    group.GET("/public-status", p.handlePublicStatus) // 命中白名单，自动免鉴权放行
    group.GET("", p.handleListOrders)                 // 受保护接口，需登录鉴权
    group.POST("", p.handleCreateOrder)
    group.GET("/:id", p.handleGetOrderDetail)

    return nil
}
```
> 💡 **鉴权放行防线**：`auth` 插件提供的 `RequireAuthMiddleware()` 内部已接入白名单拦截器。所有注册到白名单的路由在经过鉴权中间件时均会自动放行，彻底杜绝免鉴权接口被全局或组级鉴权中间件误拦截（返回 401 Unauthorized）。

---

### 场景 5：如何获取当前登录用户信息？
`auth` 插件会在上下文中注入当前用户 Session。业务 Handler 可直接调用统一 Helper：

```go
func (p *OrderPlugin) handleCreateOrder(c *gin.Context) {
    // 1. 从当前 Gin 请求上下文中提取认证用户信息
    currentUser, ok := oauth.GetCurrentUser(c)
    if !ok {
        response.AbortUnauthorized(c, errs.ErrUnauthorized)
        return
    }

    log.Printf("当前下单用户 ID: %s, 权限角色: %s", currentUser.ID, currentUser.Role)
    // 2. 正常业务处理...
}
```

---

### 场景 6：如何开发并注册一个 Asynq 异步 Worker 任务？
```go
func (p *OrderPlugin) Apply(ctx *core.Context) error {
    // 1. 注册 Asynq 任务类型与消费处理器
    ctx.Task().Register("order:cancel_timeout", p.handleTimeoutCancelTask)
    return nil
}

// 2. 任务执行函数
func (p *OrderPlugin) handleTimeoutCancelTask(ctx context.Context, t *asynq.Task) error {
    var payload OrderTimeoutPayload
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return err
    }
    // 执行超时关单业务逻辑...
    return nil
}

// 3. 业务中异步投递任务
func (p *OrderPlugin) EnqueueTimeoutCheck(ctx context.Context, orderID string) {
    p.ctx.TaskClient().EnqueueContext(ctx, asynq.NewTask("order:cancel_timeout", payloadBytes), asynq.ProcessIn(15*time.Minute))
}
```

---

### 场景 7：如何开发并注册一个 Cron 定时任务？
```go
func (p *ReportPlugin) Apply(ctx *core.Context) error {
    // 每天凌晨 2 点执行日报汇总任务
    ctx.Schedule().RegisterCron("0 2 * * *", "report:daily_summary", DailyReportPayload{Type: "all"})
    return nil
}
```

---

### 场景 8：数据库表结构如何声明？ORM 模型规范是什么？
**规范**：
1. 表名必须带有插件专有前缀（如 `w_order_`、`w_auth_`），避免跨插件表名冲突。
2. 零值与数据库默认值严格对齐；禁止物理外键，显式建索引。
3. 必须通过 GORM 结构体清晰声明 `gorm:"..."` 标签与 `json:"..."`。

```go
package models

import "time"

type Order struct {
    ID        string     `gorm:"column:id;primaryKey;size:64" json:"id"`
    UserID    string     `gorm:"column:user_id;index;size:64;not null" json:"user_id"`
    Amount    int64      `gorm:"column:amount;not null" json:"amount"`
    Status    string     `gorm:"column:status;size:32;index;not null;default:'pending'" json:"status"`
    CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
    UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
    DeletedAt *time.Time `gorm:"column:deleted_at;index" json:"-"`
}

func (Order) TableName() string {
    return "w_orders"
}
```

---

### 场景 9：数据库如何做独立迁移？Goose SQL 怎么组织？
**彻底告别集中大迁移目录**。每个插件在内部目录建立 `migrations/`，并通过 `//go:embed` 打包注入：

```go
// plugins/order/plugin.go
package order

import (
	"embed"
	"github.com/Rain-kl/Wavelet/core"
)

//go:embed migrations/*.sql
var orderMigrations embed.FS

func (p *Plugin) Apply(ctx *core.Context) error {
	// 注册本插件的专属迁移（系统启动时自动按版本号执行）
	ctx.Migrations().Register("order", orderMigrations)
	return nil
}
```

#### SQL 迁移脚本规范 (`plugins/order/migrations/00001_initial.sql`)：

每个插件只需维护一个 `00001_initial.sql`，包含该插件的全部建表语句与种子数据。

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_orders (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_orders_user_id ON w_orders(user_id);

-- 种子数据（使用 ON CONFLICT DO NOTHING 保证幂等）
INSERT INTO w_orders (id, user_id, amount, status, created_at, updated_at)
VALUES ('init_001', 'system', 0, 'completed', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_orders;
-- +goose StatementEnd
```

#### 版本管理机制

所有插件共享一张 `w_schema_versions` 表，以 `plugin_id` 区分：

```
w_schema_versions (plugin_id, version_id, applied_at)
```

启动时，引擎遍历每个插件：
1. 查询 `w_schema_versions WHERE plugin_id = 'order'` 获取当前最大版本号
2. 扫描插件 `migrations/` 目录下的 `.sql` 文件
3. 如果存在未应用的版本号 → 执行迁移
4. 如果全部已应用 → 跳过

```sql
-- 查看全局迁移状态
SELECT * FROM w_schema_versions ORDER BY plugin_id, version_id;
```

---

### 场景 10：如果有多个业务插件需要读写同一张表怎么办？
**黄金准则**：**表有且仅有一个所有者插件 (Single Owner Principle)**。
* 严禁插件 B 直接通过 SQL 修改插件 A 拥有的核心表（如订单插件直接修改用户表）。
* **合法模式 1（服务调用）**：插件 A 提供 `UserService.DeductBalance(uid, amount)`，插件 B 调用该接口。
* **合法模式 2（只读视图 / 共享查询 DTO）**：如果仅仅是高频联合查询（报表），插件 A 暴露只读查询接口，或通过数据库只读从库直接投影。

---

### 场景 11：如果跨插件操作多张表，如何确保事务一致性？
在插件化和微服务就绪体系下，**跨插件的强分布式事务是反模式**。

1. **同插件内多表操作**：直接使用本地数据库事务：
   ```go
   err := ctx.DB().Transaction(func(tx *gorm.DB) error {
       if err := tx.Create(&order).Error; err != nil { return err }
       if err := tx.Create(&orderItem).Error; err != nil { return err }
       return nil
   })
   ```
2. **跨插件操作（如创建订单 + 扣减库存 + 发送通知）**：
   * 采用 **最终一致性 (Eventual Consistency / Saga 模式)**。
   * 本地事务成功后，发射 `OrderCreatedEvent` 到 EventBus；
   * 库存插件监听到事件后扣减库存，若失败则发布补偿事件触发订单取消。

---

### 场景 12：如何发布和订阅领域事件 (EventBus)？
Wavelet 完整实现了 Cordis 架构的 **4 种类型化事件分发语义**，支持同步管道、并发聚合与异步广播：

| 分发方法 | 分发语义 | 返回值/错误处理 | 适用场景 |
| :--- | :--- | :--- | :--- |
| `ctx.Events().Emit(ctx, topic, payload)` | **异步广播 (Broadcast)** | 不阻塞主流程，静默恢复 handler panic | 状态变更广播、审计日志记录、跨插件解耦通知 |
| `ctx.Events().Waterfall(ctx, topic, initial)` | **流式管道 (Waterfall)** | 依次将前一个 handler 返回值传给下一个，遇错立即短路退出 | 参数过滤拦截链、内容审查、数据清洗与加工 |
| `ctx.Events().Parallel(ctx, topic, payload)` | **并发聚合 (Parallel)** | 并发启动 goroutine 执行所有 handler，用 `errors.Join` 聚合所有错误 | 并发外部系统推送、多渠道并行通知校验 |
| `ctx.Events().Serial(ctx, topic, payload)` | **串行执行 (Serial)** | 按注册顺序依次同步执行，遇到首个非 nil 错误立即短路中断 | 敏感操作前置拦截（如权限/风控准入校验） |

#### 1. 异步广播 (`Emit`) 与订阅 (`On`)
```go
// 1. 定义强类型事件结构
type OrderPaidEvent struct {
    OrderID   string `json:"order_id"`
    UserID    string `json:"user_id"`
    PayAmount int64  `json:"pay_amount"`
}

// 2. 插件 A 发布事件（广播）
ctx.Events().Emit(ctx, "order:paid", OrderPaidEvent{OrderID: "ord_1", UserID: "u_1", PayAmount: 9900})

// 3. 插件 B 订阅事件
ctx.Events().On("order:paid", func(c context.Context, e OrderPaidEvent) error {
    log.Printf("收到支付成功事件，开始为用户 %s 发放权益", e.UserID)
    return nil
})
```

#### 2. 流式管道变换 (`Waterfall`)
Handler 支持返回 `(T, error)` 或 `T`，后续 Handler 接收上一 Handler 的返回值：
```go
// 插件注册拦截处理
ctx.Events().On("content:filter", func(c context.Context, text string) (string, error) {
    return strings.ReplaceAll(text, "敏感词", "***"), nil
})

// 调用方通过 Waterfall 获得流式处理后的结果
cleaned, err := ctx.Events().Waterfall(ctx, "content:filter", "原始文本包含敏感词")
// cleaned == "原始文本包含***"
```

#### 3. 串行准入拦截 (`Serial`)
```go
// 风控插件注册校验
ctx.Events().On("order:pre_create", func(c context.Context, req CreateOrderRequest) error {
    if isBlacklisted(req.UserID) {
        return errors.New("用户处于风控黑名单，禁止下单")
    }
    return nil
})

// 订单插件执行准入链，遇到首个错误立即中断并返回
if err := ctx.Events().Serial(ctx, "order:pre_create", req); err != nil {
    return err // 拦截创建
}
```

---

### 场景 13：如何向系统注册插件自定义配置（config.yaml 与管理台热加载设置）？
```go
type OrderConfig struct {
    MaxItemsPerOrder int  `yaml:"max_items" json:"max_items"`
    AutoCancelMins   int  `yaml:"auto_cancel_mins" json:"auto_cancel_mins"`
}

func (p *OrderPlugin) Apply(ctx *core.Context) error {
    var cfg OrderConfig
    // 1. 自动从 config.yaml 中的 plugins.order 节点绑定配置
    ctx.Config().Bind("plugins.order", &cfg)
    
    // 2. 注册为管理台可动态修改的系统参数
    ctx.Settings().Register(core.SettingSchema{
        Key:         "order.auto_cancel_mins",
        Default:     15,
        Description: "未支付订单自动取消时间 (分钟)",
    })
    return nil
}
```

---

### 场景 14：如何使用多层缓存（RAM L1 + Redis L2 + PubSub 同步）？
框架提供三层穿透缓存能力，防止缓存击穿与雪崩：

```go
func (s *OrderService) GetOrderWithCache(ctx context.Context, orderID string) (*Order, error) {
    var order Order
    err := s.ctx.Cache().GetOrSet(ctx, "order:"+orderID, &order, 10*time.Minute, func() (any, error) {
        // Cache Miss 回源查 DB
        var dbOrder Order
        if err := s.db.WithContext(ctx).First(&dbOrder, "id = ?", orderID).Error; err != nil {
            return nil, err
        }
        return &dbOrder, nil
    })
    return &order, err
}

// 当订单更新时，广播失效所有节点的 L1 内存缓存与 L2 Redis 缓存
func (s *OrderService) InvalidateCache(ctx context.Context, orderID string) {
    s.ctx.Cache().Delete(ctx, "order:"+orderID)
}
```

---

### 场景 15：如何使用分布式锁 (DistLock) 防止并发超卖与重复消费？
```go
func (s *OrderService) ProcessPayment(ctx context.Context, orderID string) error {
    // 获取分布式锁，租期 5 秒
    unlock, err := s.ctx.DistLock().Lock(ctx, "lock:order:pay:"+orderID, 5*time.Second)
    if err != nil {
        return fmt.Errorf("当前订单正在处理中，请勿重复提交")
    }
    defer unlock() // 确保释放

    // 执行扣款操作...
    return nil
}
```

---

### 场景 16：如何向管理后台动态注册监控数据与管理控制台？
插件可以向管理后台扩展点注入自己的仪表盘指标和诊断探针：

```go
func (p *OrderPlugin) Apply(ctx *core.Context) error {
    ctx.Admin().RegisterMetric("order_count_today", func(c context.Context) any {
        var count int64
        ctx.DB().Model(&models.Order{}).Where("created_at >= ?", todayStart()).Count(&count)
        return count
    })
    return nil
}
```

---

### 场景 17：插件如何实现健康检查探针与就绪检查 (Health Check)？
```go
func (p *PaymentPlugin) Apply(ctx *core.Context) error {
    ctx.Health().RegisterProbe("payment_gateway", func(ctx context.Context) error {
        // 测试第三方支付网关网络连通性
        return pingPaymentGateway(ctx)
    })
    return nil
}
```

---

### 场景 18：插件如何扩展其他插件的能力（如新增一种 OAuth 登录提供商 / 新增消息推送渠道）？
采用 **注册表扩展点模式 (Registry Pattern)**：

```go
// 1. 下游编写微信登录插件 plugins/oauth_wechat
func (p *WeChatOAuthPlugin) Apply(ctx *core.Context) error {
    return ctx.Using(func(authRegistry contracts.AuthRegistry) {
        // 向核心 auth 插件注入微信 OAuth 实现
        authRegistry.RegisterOAuthProvider("wechat", &WeChatProvider{...})
    })
}
```

---

### 场景 19：插件如何编写单元测试与集成测试（Mock 上下文与依赖打桩）？
微内核提供轻量测试脚手架 `coretest`：

```go
func TestOrderCreate(t *testing.T) {
    // 1. 创建内存测试专用 Context
    ctx := coretest.NewMockContext(t)
    
    // 2. Mock 依赖的 UserService
    mockUserSvc := &MockUserService{ReturnUser: &contracts.UserDTO{ID: "u_1", Balance: 1000}}
    ctx.Provide[contracts.UserService](mockUserSvc)

    // 3. 装载插件
    plugin := &OrderPlugin{}
    require.NoError(t, plugin.Apply(ctx))

    // 4. 发起 HTTP 接口测试
    w := ctx.PerformRequest("POST", "/api/v1/orders", `{"item_id":"item_1"}`)
    assert.Equal(t, 200, w.Code)
}
```

---

### 场景 20：以不同角色（api / worker / schedule / all）启动时，插件代码如何适配？
**开发者无需做任何特殊处理**！
插件只需在一个 `Apply` 方法中把自己的路由、任务、调度全部注册进 `Context`。微内核调度器会根据运行命令自动按需激活对应的运行时驱动，不匹配的能力保持休眠。

---

### 场景 21：当某个插件流量暴增需要独立拆分为微服务时，如何零成本平滑改造？
```go
// 1. 之前单体模式：在 main.go 中加载本地实现
app.Use(&auth.Plugin{}) // 进程内直接运行

// 2. 拆分为微服务后：只需将 main.go 替换为 gRPC 客户端代理插件！
app.Use(&auth_grpc_client.Plugin{RemoteAddr: "auth-service.prod:9000"})

// 3. 所有依赖 auth 的业务插件（如 order, user）业务代码 0 处修改！
```

---

### 场景 22：插件如何安全处理文件上传与大文件摄取 (upload.Ingest)？
**严格规则**：禁止插件自行直接写入对象存储底层 Bucket 或直连底层文件系统。统一走平台摄取服务：

```go
func (p *OrderPlugin) handleUploadInvoice(c *gin.Context) {
    fileHeader, _ := c.FormFile("file")
    
    // 使用平台统一摄取引擎（自动计算哈希、防重传、生成签名 URL 与入库追踪）
    ingestResult, err := upload.IngestFormFile(c.Request.Context(), fileHeader, upload.IngestPolicy{
        AllowedTypes: []string{"image/png", "application/pdf"},
        MaxSizeBytes: 10 * 1024 * 1024,
    })
    if err != nil {
        response.AbortBadRequest(c, errs.ErrUploadFailed)
        return
    }

    c.JSON(200, response.OK(gin.H{"file_url": ingestResult.URL}))
}
```

---

# 第二部分：整个项目的目录结构划分与包职责定义

```text
Wavelet/
├── cmd/                          # CLI 命令分发与装配入口
│   ├── root.go                   # Cobra 根命令
│   ├── server.go                 # 综合启动器（支持 api/worker/schedule/all profile）
│   └── migrate.go                # 数据库独立迁移命令行工具
│
├── core/                         # 【微内核引擎 (Zero Business Logic)】
│   ├── context.go                # Context 上下文总线与 Fork 树
│   ├── container.go              # 基于泛型的 IoC 服务注册与解析器
│   ├── events.go                 # 强类型领域事件总线 (EventBus)
│   ├── lifecycle.go              # 启动/停止生命周期编排状态机
│   ├── contracts/                # 【跨插件标准服务契约 (纯 Interface)】
│   │   ├── auth.go               # AuthService 契约
│   │   ├── user.go               # UserService 契约
│   │   ├── cache.go              # CacheService 契约
│   │   └── database.go           # DBService 契约
│   └── extpoints/                # 扩展点定义 (Router, Task, Migration, Setting)
│
├── plugins/                      # 【官方标准插件库 (完全高内聚闭包)】
│   ├── drivers/                  # 运行时驱动插件
│   │   ├── driver_http/          # Gin Web HTTP 驱动
│   │   ├── driver_asynq_worker/  # Asynq Worker 并发消费驱动
│   │   └── driver_asynq_cron/    # Asynq Cron 调度器驱动
│   │
│   ├── infra/                    # 基础设施服务插件
│   │   ├── database/             # GORM 多数据源与读写分离插件
│   │   ├── cache/                # RAM + Redis + PubSub 缓存插件
│   │   ├── logger/               # Zap + Otel 分布式链路追踪日志插件
│   │   └── storage/              # S3 / OSS / Local 对象存储插件
│   │
│   └── domain/                   # 业务领域能力插件
│       ├── auth/                 # OAuth / Session / Passkey 认证插件
│       ├── user/                 # 用户资料 / 权限 / 角色插件
│       ├── message_gateway/      # Bot 网关 / 渠道推送插件
│       ├── risk_control/         # 访问控制 / IP 限流 / 安全风控插件
│       └── admin/                # 系统管理台与监控面板插件
│
└── downstream/                   # 【下游二开项目模板与脚手架】
    ├── README.md                 # 下游插件开发指南
    └── plugins/                  # 下游自定义业务插件目录
        └── custom_example/       # 官方标准插件开发基准模板（含完整分层结构）
```

### 各层职责与禁止规则 (Guardrails)：
1. **`core/`**：
   - **职责**：纯抽象，提供 IoC、Context、EventBus 和 Lifecycle。
   - **严禁**：严禁 import 任何具体业务包，严禁 import `gin`、`gorm`、`asynq`。
2. **`core/contracts/`**：
   - **职责**：仅定义公开的 Go Interface 和公共 DTO。
   - **严禁**：严禁包含任何具体实现逻辑或 SQL 操作。
3. **`plugins/` 与 `downstream/`**：
   - **职责**：所有业务逻辑和驱动实现的归宿。遵循统一标准分层架构（Layered Architecture / MVC 变体）。
   - **统一分层架构与标准开发模板**：
     - **唯一基准模板**：以 [`backend/downstream/plugins/custom_example`](file:///Users/ryan/Code/Go/Wavelet/backend/downstream/plugins/custom_example) 为全项目统一基准模板。
     - **物理子包隔离结构**：包含 `plugin.go`, `consts/`, `controller/`, `service/`, `dao/`, `model/` [含 `entity/`, `do/`], `migrations/` [含 `postgres/`, `sqlite/`]。
     - **命名与依赖禁令**：**严禁在根包平铺 `handlers_*`、`service_*`、`dao_*` 等前缀文件**，子包内文件直接以纯业务实体命名（如 `hello.go`、`user.go`）。严格约束 `controller -> service -> dao -> model` 单向依赖。
   - **严禁**：插件之间严禁跨包 import 内部私有代码，跨插件调用一律走 `contracts` 接口或 `EventBus`。

---

# 第三部分：框架核心提供给插件调用的公用能力矩阵 (Context Capability Matrix)

每个插件在 `Apply(ctx *core.Context)` 时，都可以无缝调用微内核暴露的以下标准能力：

| 扩展点/能力方法 | 返回类型 | 功能说明 | 适用场景 |
| :--- | :--- | :--- | :--- |
| `ctx.Router()` | `RouterExtension` | 声明 HTTP 路由、前缀分组、挂载中间件与免鉴权白名单注册（`RegisterWhitelist`, `IsWhitelisted`），支持 `Unregister` / `UnregisterByID` | 暴露 API 接口、公开免鉴权端点、Web 控制台 |
| `ctx.Task()` | `TaskExtension` | 注册 Asynq 异步任务消费处理器，支持 `Unregister` | 耗时后台任务、异步消息发送 |
| `ctx.Schedule()` | `ScheduleExtension`| 注册 Cron 定时调度任务，支持 `Unregister` | 定时报表统计、周期性清理 |
| `ctx.Migrations()` | `MigrationExtension`| 注册插件专属的 Goose SQL 迁移嵌入系统，支持 `Unregister` | 自建数据表、版本升级 |
| `ctx.Events()` | `EventBus` | 强类型领域事件总线（支持 `Emit`, `Waterfall`, `Parallel`, `Serial`） | 跨插件完全解耦通知与状态同步 |
| `ctx.Settings()` | `SettingExtension` | 声明动态可配置项（支持热更新），支持 `Unregister` | 业务参数配置、管理台可调节参数 |
| `ctx.Fork()` | `*Context` | 创建继承父级容器并隔离局部副作用的子上下文 | 局部 Fiber、请求域隔离 |
| `core.Provide[T]`| `void` | 向全局 IoC 容器注册强类型服务（自动挂载 `OnDispose` 逆操作） | 暴露自身能力给其他插件消费 |
| `core.Inject[T]` | `(T, error)` | 从全局 IoC 容器中按类型获取服务实例 | 消费其他插件暴露的服务 |
| `core.When[T]` | `void` | 响应式监听服务注入（当服务一旦就绪立即触发回调） | 解决插件装载时序竞争与延迟初始化 |
| `core.Has[T]` | `bool` | 判断指定服务类型当前是否已在容器中注册 | 探测环境能力与条件装载 |
| `ctx.Using(func(T))` | `error` | 响应式声明依赖，当服务就绪时执行回调 | 声明前置依赖关系 |
| `core.Inject[contracts.DBService]` | `(DBService, error)` | 获取受事务与 Trace 保护的数据库连接与 GORM 实例 | 数据持久化 CRUD |
| `core.Inject[contracts.CacheService]` | `(CacheService, error)` | 三层穿透缓存（RAM L1 + Redis L2 + PubSub 广播）| 高频读数据性能加速 |
| `core.Inject[contracts.StorageService]` | `(StorageService, error)` | 统一对象存储读写引擎 | 文件摄取、图片持久化 |
| `core.Inject[contracts.TaskService]` | `(TaskService, error)` | 后台任务下发、重试与调度管理契约 | 任务下发与定时调度管理 |
| `core.Inject[contracts.RiskControlService]` | `(RiskControlService, error)` | 访问日志查询、聚合分析与存储引擎管理契约 | 审计日志与安全分析 |

---

# 第四部分：Cordis 插件分层开发规范与代码模板 (Plugin Layered Architecture & Code Templates)

为了统一规范 Wavelet 所有官方插件与下游业务二开插件的研发质量，全项目插件**统一以 [`backend/downstream/plugins/custom_example`](file:///Users/ryan/Code/Go/Wavelet/backend/downstream/plugins/custom_example) 为基准开发模板**，遵循物理子包隔离的标准分层架构（`controller -> service -> dao -> model`）。

## 1. 统一标准分层架构与目录规范 (以 `custom_example` 为基准)

```text
backend/downstream/plugins/custom_example/ (或 backend/plugins/domain/<plugin_name>/)
├── plugin.go              # [插件根入口] 实现 core.Plugin，负责装配依赖、路由与扩展点注册
│
├── consts/                # package consts：常量定义、配置键名与模块错误标识
│   └── consts.go
│
├── controller/            # package controller：HTTP API 接入层 (参数绑定、会话提取、信封响应)
│   └── hello/             # 业务分组子包
│       └── hello.go       # 业务接口 Handler 实现（直接以业务命名，禁止 controller_hello.go）
│
├── service/               # package service：核心业务逻辑层 (业务用例、事务编排、事件发布)
│   └── hello.go           # 业务用例实现（纯 Go 逻辑，禁止依赖 *gin.Context）
│
├── dao/                   # package dao：数据持久化访问层 DAL (GORM CRUD、SQL 转义防注入)
│   └── hello.go           # 数据库访问实现（直接以业务命名，禁止 dao_hello.go）
│
├── model/                 # package model：纯数据实体与传输对象 (零 Web/数据库框架依赖)
│   ├── entity/            # 数据库映射实体 (TableName() 必须带专属表前缀)
│   │   └── hello.go
│   └── do/                # 领域对象、请求 Request DTO 与响应 Response DTO
│       └── hello.go
│
└── migrations/            # Goose SQL 独立迁移嵌入目录 (//go:embed)
    ├── postgres/          # PostgreSQL 专属迁移 SQL
    │   └── 20260901000001_init_hello.sql
    └── sqlite/            # SQLite 专属迁移 SQL
        └── 20260901000001_init_hello.sql
```

> ⚠️ **严禁规则**：
> - **严禁在根目录平铺文件**：严禁在插件根目录下创建 `handlers_*.go`、`service_*.go`、`dao_*.go` 等前缀文件。
> - **严禁跨层违规调用**：严格约束 `controller -> service -> dao -> model` 单向依赖。
> - **严禁跨插件私有导入**：跨插件调用一律走 `contracts` 接口或 `EventBus`。

---

## 2. 核心分层代码模板与实现范例

#### (1) `plugin.go` (插件装配入口)
```go
package hello

import (
	"embed"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/gin-gonic/gin"
)

//go:embed migrations/postgres/*.sql
var pgMigrations embed.FS

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return "custom_example"
}

func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. 注册专属数据库迁移
	ctx.Migrations().Register("custom_example", pgMigrations)

	// 2. 解析依赖并装配各层
	var authSvc contracts.AuthService
	if err := core.Using[contracts.AuthService](ctx, func(svc contracts.AuthService) { authSvc = svc }); err != nil {
		return err
	}

	// 3. 注册 HTTP 路由组与中间件
	g := ctx.Router().Group("/api/v1/custom", authSvc.RequireAuthMiddleware().(gin.HandlerFunc))
	g.GET("/hello", func(c *gin.Context) {
		user, err := authSvc.GetCurrentUser(c.Request.Context())
		if err != nil {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(200, gin.H{"message": "Hello " + user.Username})
	})

	return nil
}
```

#### (2) `controller/hello/hello.go` (HTTP 控制器层)
```go
package hello

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/pkg/oauth"
	"github.com/Rain-kl/Wavelet/pkg/response"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	svc HelloService
}

func NewController(svc HelloService) *Controller {
	return &Controller{svc: svc}
}

// @Summary 获取欢迎信息
// @Tags Hello
// @Produce json
// @Success 200 {object} response.Envelope{data=do.HelloResponse} "成功"
// @Router /api/v1/custom/hello [get]
func (ctrl *Controller) GetHello(c *gin.Context) {
	user, ok := oauth.GetCurrentUser(c)
	if !ok {
		response.AbortUnauthorized(c, "errUnauthorized")
		return
	}

	res, err := ctrl.svc.SayHello(c.Request.Context(), user.ID)
	if err != nil {
		response.AbortInternal(c, "errInternalServer")
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}
```

#### (3) `service/hello.go` (业务逻辑层)
```go
package service

import (
	"context"

	"github.com/Rain-kl/Wavelet/core"
)

type HelloService struct {
	ctx *core.Context
	dao HelloDAO
}

func NewHelloService(ctx *core.Context, dao HelloDAO) *HelloService {
	return &HelloService{ctx: ctx, dao: dao}
}

func (s *HelloService) SayHello(ctx context.Context, userID string) (*do.HelloResponse, error) {
	record, err := s.dao.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 发射领域事件
	s.ctx.Events().Emit(ctx, "custom:hello_visited", map[string]any{"user_id": userID})

	return &do.HelloResponse{Message: "Hello " + record.Name}, nil
}
```

#### (4) `dao/hello.go` (数据访问持久化层 DAL)
```go
package dao

import (
	"context"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/pkg/util"
	"gorm.io/gorm"
)

type HelloDAO struct {
	ctx *core.Context
}

func NewHelloDAO(ctx *core.Context) *HelloDAO {
	return &HelloDAO{ctx: ctx}
}

func (d *HelloDAO) getDB(ctx context.Context) *gorm.DB {
	if dbSvc, err := core.Inject[contracts.DBService](d.ctx); err == nil && dbSvc != nil {
		return dbSvc.GetDB().WithContext(ctx)
	}
	return nil
}

func (d *HelloDAO) GetByUserID(ctx context.Context, userID string) (*entity.HelloEntity, error) {
	var item entity.HelloEntity
	err := d.getDB(ctx).Where("user_id = ?", userID).First(&item).Error
	return &item, err
}

func (d *HelloDAO) SearchByKeyword(ctx context.Context, keyword string) ([]entity.HelloEntity, error) {
	var list []entity.HelloEntity
	// SQL LIKE 防注入与通配符转义规范
	safeKeyword := util.EscapeLike(keyword) + "%"
	err := d.getDB(ctx).Where("name LIKE ? ESCAPE '\\'", safeKeyword).Find(&list).Error
	return list, err
}
```

#### (5) `model/entity/` 与 `model/do/`
```go
// model/entity/hello.go
package entity

import "time"

type HelloEntity struct {
	ID        string    `gorm:"column:id;primaryKey;size:64" json:"id"`
	UserID    string    `gorm:"column:user_id;index;size:64;not null" json:"user_id"`
	Name      string    `gorm:"column:name;size:128;not null" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (HelloEntity) TableName() string {
	return "w_custom_hello"
}
```

```go
// model/do/hello.go
package do

type HelloResponse struct {
	Message string `json:"message"`
}

type CreateHelloRequest struct {
	Name string `json:"name" binding:"required,max=128"`
}
```

#### (6) `consts/consts.go` (常量与错误定义)
```go
package consts

const (
	PluginName = "custom_example"

	// 模块内部错误码标识 (camelCase)
	ErrUserNotFound   = "errUserNotFound"
	ErrInvalidParams  = "errInvalidParams"
	ErrOperationFail  = "errOperationFail"
)
```

---

## 3. 各层核心职责边界与严格禁止防线 (Guardrails)

```text
┌──────────────────────────────────────────────────────────────────┐
│                   Controller 层 (HTTP API 接入)                  │
│   • 参数绑定 ShouldBindJSON        • 用户会话 oauth.GetCurrentUser │
│   • 统一信封 response.OK/Abort*    • 严禁 SQL 操作 / 严禁重度业务   │
└─────────────────────────────────┬────────────────────────────────┘
                                  │ 调用 Service (入参首位 context.Context)
                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│                     Service 层 (业务用例 & 领域逻辑)             │
│   • 纯 Go 逻辑 (零 Web 依赖)      • 事务编排 ctx.DB().Transaction │
│   • 领域事件 ctx.Events().Emit     • 严禁 import gin / c.JSON     │
└─────────────────────────────────┬────────────────────────────────┘
                                  │ 调用 DAO 接口
                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│                      DAO 层 (数据持久化 DAL)                     │
│   • GORM CRUD 与查询              • EscapeLike 通配符安全转义     │
│   • 严禁反向依赖 Service/Controller • 严禁越权读写其他插件数据表  │
└─────────────────────────────────┬────────────────────────────────┘
                                  │ 映射
                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│                     Model 层 (纯实体 & DTO)                      │
│   • TableName() 带专属表前缀        • entity/ 与 do/ 明确拆分      │
│   • 零值与 DB 默认值匹配           • 无任何上层包依赖             │
└──────────────────────────────────────────────────────────────────┘
```

1. **表单一所有者原则 (Single Owner Principle)**：数据表有且仅由所属插件声明与维护（表名统一前缀 `w_<plugin>_*`），跨插件一律通过公开契约 Interface 或 EventBus 协同。
2. **LIKE 查询安全防注入**：所有涉及用户输入的模糊查询，必须经过 `util.EscapeLike` 转义通配符并显式声明 `ESCAPE '\\'` 语法。
3. **Goroutine 安全**：并发任务统一使用 `util.Go`，杜绝直接使用裸 `go func()`。

