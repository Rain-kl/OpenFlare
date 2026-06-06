# 登录集成 Cap 验证码实现计划

本计划规定了在 OpenFlare 系统的登录流程中集成 Cap（基于 Proof-of-Work 和无感浏览器检测的验证码）的具体开发步骤。

---

## 1. 目标与背景 (Goal & Context)

### 需求背景
为解决安全分析中识别到的“登录接口缺少防暴力破解/撞库逻辑”这一安全风险，我们需要在登录接口中集成 Cap 验证码服务。通过让客户端（爬虫/浏览器）在登录前必须求解一个 PoW 工作量难题并核销，显著提高恶意爬虫爆破的计算成本，从根本上防止针对登录接口的恶意爆破。

### 开发范围
1. **后端验证服务**：在 Server 端移植 `capjs-core` 的 PoW 校验算法（包括 FNV-1a、自定义 PRNG、SHA-256 检验、JWT 难题派发与核销缓存组件）。
2. **公开路由映射**：
   * `POST /api/cap/challenge` (分发难题)
   * `POST /api/cap/redeem` (核销难题并核发 `cap-token`)
3. **控制开关**：增加全局选项 `CapLoginEnabled`，管理员可动态启停。
4. **前端交互接入**：在登录页面引入 `cap-widget` 自定义组件，并在提交登录请求时附带 `cap_token`。

---

## 2. 设计与决策 (Design & Decisions)

### 核心对象与数据模型
本方案不涉及复杂数据库结构重构，但需要：
1. 在 `options` 表中保存 `CapLoginEnabled` (true/false) 选项, 默认为 True, 设置路径在 设置->系统设置->登录与注册开关。
2. 建立一个全局的、线程安全的内存验证码核销存储/核销缓存，具备过期清理功能，用于存放核销的 `cap-token` 以及消费过的 JWT 难题 Nonce（Signature），支持 Redis 与本地内存模式。

### API 与鉴权设计
1. **`POST /api/cap/challenge`**：公开接口。
2. **`POST /api/cap/redeem`**：公开接口。
3. **`POST /api/user/login`**：接受可选/必选的 `cap_token` 参数。

### 算法移植 (Proof-of-Work Go 实现)
* FNV-1a 状态机复现。
* 伪随机数生成器 (PRNG) 与 `strings.HasPrefix(sha256Hex, target)` 校验。

---

## 3. 具体修改文件清单 (Proposed Changes)

### 后端 Server
* #### [MODIFY] [constants.go](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/common/constants.go)
  * 增加 `CapLoginEnabled` 全局常量/变量，默认 `true`。
* #### [MODIFY] [option.go](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/model/option.go)
  * 在 `InitOptionMap` 和 `updateOptionMap` 中添加 `CapLoginEnabled` 的支持。
* #### [NEW] [prng.go (utils/cap)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/utils/cap/prng.go)
  * 职责：实现 FNV-1a、FNV-1a resume 及 XORShift-based 自定义 PRNG 伪随机数算法。
* #### [NEW] [cap.go (utils/cap)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/utils/cap/cap.go)
  * 职责：实现无状态 PoW 难题生成、验证及 JWT 校验。
* #### [NEW] [store.go (utils/cap)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/utils/cap/store.go)
  * 职责：定义 `Store` 接口并提供默认的高性能、线程安全的内存 TTL 缓存核销存储实现。
* #### [NEW] [manager.go (utils/cap)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/utils/cap/manager.go)
  * 职责：封装验证码的核心逻辑，暴露出 `Generate`、`Redeem` 与 `VerifyToken` 高阶 API。
* #### [NEW] [middleware.go (utils/cap)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/utils/cap/middleware.go)
  * 职责：实现通用的 Gin 中间件 `VerifyMiddleware`。其不依赖任何 OpenFlare 业务代码，完全通过构造注入。
* #### [NEW] [cap.go (service)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/service/cap.go)
  * 职责：适配器服务，将 OpenFlare 的全局参数（如 `JWTSecret`、`CapLoginEnabled`、`RDB`）注入并实例化全局的 `CapManager` 实例。
* #### [NEW] [cap.go (middleware)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/middleware/cap.go)
  * 职责：极简的适配器中间件，直接调用并返回 `service.CapManager.VerifyMiddleware(scope)`。
* #### [NEW] [cap.go (controller)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/controller/cap.go)
  * 职责：实现 `GetCapChallenge` 和 `RedeemCapChallenge` 控制器。
* #### [MODIFY] [api-router.go](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/router/api-router.go)
  * 职责：挂载 `/api/cap/challenge` 和 `/api/cap/redeem` 路由，并在 `/api/user/login` 上应用 `middleware.CapAuth("login")`。
* #### [MODIFY] [user.go (controller)](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/controller/user.go)
  * 无需修改：登录控制器和入参结构体保持完全无侵入。
* #### [MODIFY] [misc.go](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/controller/misc.go)
  * 职责：在 `GetStatus` 中返回 `cap_login_enabled` 开关状态。

### 前端 Web
* #### [MODIFY] [public-status.ts](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/web/types/public-status.ts)
  * 添加 `cap_login_enabled: boolean` 字段。
* #### [MODIFY] [auth.ts](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/web/types/auth.ts)
  * 在 `LoginPayload` 中添加可选的 `cap_token?: string` 属性。
* #### [MODIFY] [login-form.tsx](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/web/features/auth/components/login-form.tsx)
  * 动态载入 `cap-widget`（脚本 CDN：`https://cdn.jsdelivr.net/npm/cap-widget`）。
  * 若后台返回 `cap_login_enabled === true`，则渲染 `<cap-widget data-cap-api-endpoint="/api/cap/" />` 组件。
  * 在表单提交时，将 `cap-token` 塞入 `loginMutation` 的 Payload 中提交。

---

## 4. 验证计划 (Verification Plan)

### 自动化单元测试
* 针对 Go 中的 PoW 核心算法，编写单测 `openflare-server/service/cap_test.go`。
* 运行单测命令：`go test -v ./openflare-server/service/...`

### 手动功能与防暴力破解验证
1. 打开控制台选项开启 `CapLoginEnabled`。
2. 访问登录页面，观察人机验证组件静默加载并完成 PoW 计算，输入正确账户成功登录。
3. 使用 `curl` 模拟恶意爬虫不携带或携带错误的 `cap_token` 对登录 API 发起 POST 请求，预期被拦截并返回“验证码错误”。
4. 使用已被核销的同一 `cap_token` 二次请求登录，验证防重放失效机制。
