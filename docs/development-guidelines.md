# 模板工程开发规范

本文档描述当前模板工程改造阶段的开发基线。

如果需求超出 `docs/design.md` 和 `docs/template-refactor-plan.md` 的边界，必须先更新设计与计划文档，再进入实现。

## 1. 技术基线

### 1.1 Server

服务端继续基于：

* Go 1.24+
* Gin
* GORM
* SQLite / PostgreSQL
* 现有 Session 登录体系

### 1.2 Frontend

前端基线以 `openflare_server/web` 为准：

* Next.js 15 App Router
* React 19
* TypeScript
* Tailwind CSS 4
* TanStack Query
* React Hook Form + Zod
* Zustand 仅用于轻量客户端 UI 状态

前端细则见 [docs/frontend-development-guidelines.md](./frontend-development-guidelines.md)。

### 1.3 Agent

`openflare_agent` 已不再作为长期保留模块。

要求：

* 模板工程阶段不再新增 Agent 能力
* 如需调整 Agent，只允许为平稳删除、迁移或清理残留依赖服务

## 2. 工程结构规范

本次改造不仅是业务裁剪，还要同步推进工程规范化与目录结构重组。服务端应逐步迁移到以下标准 Go 工程布局：

```text
project/
├── cmd/
├── internal/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── model/
│   ├── dto/
│   ├── middleware/
│   └── pkg/
└── pkg/
```

说明：

* `cmd/`：应用入口
* `internal/handler/`：HTTP 处理器
* `internal/service/`：业务逻辑层
* `internal/repository/`：数据访问层
* `internal/model/`：领域模型/数据模型
* `internal/dto/`：数据传输对象
* `internal/middleware/`：中间件
* `internal/pkg/`：项目内公共能力，如 `utils/`、`database/`
* `pkg/`：仅在存在明确对外复用诉求时使用

要求：

* 新增代码优先落到目标目录结构
* 裁剪过程中不继续扩张旧式平铺目录
* 目录迁移要和模块裁剪、MVC 分层同时推进

## 3. MVC 分层约束

服务端必须严格遵循 MVC 扩展版分层：

* Handler：请求入参解析、权限入口控制、调用 service、输出统一响应
* Service：业务规则、流程编排、事务边界
* Repository：数据库读写与查询封装
* Model：领域对象、数据库模型表达
* DTO：请求/响应载荷和内部传输对象

禁止：

* 在 Handler 中堆积业务逻辑
* 在 Middleware 中实现具体业务流程
* 在 Router 中实现业务判断
* 在 Model 中编排跨模块业务流程
* 直接在 Handler 中替代 Repository 或 Service

要求：

* Handler 只负责接口层逻辑
* Service 负责核心业务编排
* Repository 负责持久化访问细节
* Model 负责结构表达，不承载接口逻辑
* DTO 负责传输对象定义，避免直接暴露数据库模型到接口层

## 4. 模块裁剪约束

当前模板工程保留的基础模块：

* 用户模块
* 版本升级模块
* 邮箱模块
* 文件上传模块
* 安全模块

当前模板工程待移除的 OpenFlare 业务模块：

* Agent
* 节点管理与心跳同步
* 配置版本发布与分发
* OpenResty 代理规则
* 域名与证书分发
* 访问日志、观测分析、看板

要求：

* 删除业务模块时，必须同时处理后端、前端、Swagger、配置项与文档
* 接口与界面必须同步移除、同步收敛
* 不允许保留失效导航、失效页面、失效 API client、失效类型定义或 Swagger 残留说明

## 5. 数据模型与数据库规范

要求：

* 数据模型只保留模板工程当前需要的实体
* 不新增新的 OpenFlare 领域对象
* 所有涉及表结构、索引、列类型、内部元数据的修改，都必须同步处理数据库版本与迁移

### 5.1 数据库迁移

* 数据库版本号定义在服务端模型层
* 不得仅依赖 `AutoMigrate` 隐式升级存量数据库
* 每次提升数据库版本号时，必须补充显式迁移方法
* 迁移方法必须包含升级后的校验逻辑
* 启动时必须先检查数据库当前版本，再按顺序升级
* 如果迁移失败或校验失败，启动流程必须中止

## 6. API 与鉴权规范

### 6.1 API

* 管理端 API 统一使用 JSON
* 成功与失败都必须返回清晰 `message`
* 变更类接口统一使用 `POST`
* 只读接口使用 `GET`
* 删除后端接口时，必须同步删除对应前端页面、导航入口、请求封装、类型定义和 Swagger 描述

统一响应结构：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

### 6.2 鉴权

管理端：

* 继续复用现有登录、角色与 Session

安全要求：

* 不暴露远程 shell 或任意命令执行入口
* 不在日志中打印完整 Token、验证码、敏感密钥
* 不绕过统一鉴权中间件新增临时后门接口

## 7. 配置规范

要求：

* 配置项必须围绕模板工程保留模块收敛
* 删除 OpenFlare 业务模块时，必须同步删除对应环境变量、运行时配置、默认值和文档说明
* 新增配置项时，必须同步更新 `docs/app-config.md`
* 配置项变更影响部署方式时，必须同步更新 `docs/deployment.md` 和 `README.md`

## 8. 测试与交付要求

* 关键业务逻辑必须有单元测试或等效回归测试
* 模板工程裁剪时，必须验证接口删除与界面删除是否同步完成
* 目录迁移后必须至少完成一次编译、启动或等效验证
* 服务端启动、前端构建、核心保留模块可用，是每轮裁剪后的最低验收线

## 9. 文档维护要求

以下内容变化时，必须同步更新对应文档：

* 模板工程范围或系统边界变化：更新 `docs/design.md`
* 裁剪计划、阶段目标、保留/删除清单变化：更新 `docs/template-refactor-plan.md`
* 开发约束、接口约定、MVC 分层规则、目录结构规则变化：更新本文档
* 前端工程约束变化：更新 `docs/frontend-development-guidelines.md`
* 配置项或部署方式变化：更新 `docs/app-config.md`、`docs/deployment.md` 和 `README.md`
