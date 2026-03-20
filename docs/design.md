# GinNextTemplate 设计基线

本文档定义 `GinNextTemplate` 当前阶段仍然有效的产品范围、系统边界、整体架构与长期约束。

当前项目主线已从 OpenFlare 业务产品切换为“可复用模板工程”。如果新增需求超出本文档边界，必须先更新本文档，再进入实现。

## 1. 产品定位

`GinNextTemplate` 是一个面向后台管理类项目的基础模板工程，目标是提供一套可复用、可扩展、便于继续二次开发的全栈基础底座。

当前模板保留的核心能力：

* 用户与认证
* 邮箱能力
* 文件上传
* 安全能力
* 服务端版本升级
* 基础管理端前端框架

当前模板的核心价值：

* 为新的 Gin + Next.js 项目提供开箱即用的起点
* 提供后端 MVC 分层与标准 Go 目录结构
* 提供统一的配置、鉴权、上传、邮件与升级能力
* 降低新业务项目从零搭建基础设施的成本

## 2. 当前范围边界

当前明确保留：

* 用户注册、登录、Session、权限与用户管理
* 邮箱验证码、密码重置、邮箱绑定等通用邮箱流程
* 文件上传、删除、下载、元数据管理
* 限流、验证码、安全校验、密码散列等安全基础设施
* 服务端版本检查、手动上传升级包、服务端升级链路
* 后台管理端的布局、导航、公共请求层和通用页面能力

当前明确移除或处于移除范围：

* Agent 子系统
* 节点管理与心跳同步
* 配置版本发布与分发
* OpenResty 代理规则
* 域名与证书分发
* 访问日志、观测分析、看板
* 任何围绕 OpenFlare 节点生态构建的后台页面、接口和配置项

当前明确不做：

* 多租户
* 通用工作流引擎
* 消息队列、对象存储、链路追踪等额外基础设施平台化
* 与模板主线无关的大型业务抽象

## 3. 技术基线

### 3.1 Server

服务端继续采用：

* Go 1.24+
* Gin
* GORM
* SQLite / PostgreSQL
* Session 登录体系

### 3.2 Frontend

前端继续采用：

* Next.js App Router
* React 19
* TypeScript
* Tailwind CSS

### 3.3 Agent

`openflare_agent` 不再属于模板工程长期组成部分，仅在过渡阶段用于清理和迁移。

## 4. 整体架构

```text
GinNextTemplate
├── Server (Gin + GORM + SQLite/PostgreSQL)
│   ├── Admin API
│   ├── Auth / Mail / File / Security / Upgrade
│   └── Static Web Host
└── Web Admin (Next.js)
```

职责划分：

* Server：提供管理端 API、认证、上传、邮件、安全与升级能力
* Web Admin：提供模板化后台界面与交互

## 5. 工程结构目标

本项目将逐步迁移到标准 Go 工程布局：

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

结构约束：

* `cmd/` 作为应用入口
* `internal/handler/` 作为 HTTP 接口层
* `internal/service/` 作为业务逻辑层
* `internal/repository/` 作为数据访问层
* `internal/model/` 作为领域/数据模型层
* `internal/dto/` 作为数据传输对象层
* `internal/middleware/` 作为中间件层
* `internal/pkg/` 作为项目内公共包
* `pkg/` 仅在存在明确对外复用诉求时启用

## 6. 架构原则

### 6.1 MVC 与分层约束

服务端严格遵循 MVC 扩展分层：

* Handler：请求解析、鉴权入口、响应封装
* Service：业务规则、流程编排、事务边界
* Repository：数据库访问
* Model：数据模型与领域表达
* DTO：传输对象

### 6.2 接口与界面同步收敛

删除任一业务能力时，必须同步处理：

* 后端接口
* 前端页面
* 导航入口
* API client
* 类型定义
* Swagger
* 部署文档
* 配置项说明

### 6.3 数据库迁移显式化

所有表结构、索引、列类型或内部元数据变更，都必须显式处理数据库版本与迁移，而不是仅依赖 `AutoMigrate`。

## 7. 当前核心对象

模板工程当前长期保留的核心对象应逐步收敛为：

* `users`
* `files`
* `options`
* 与升级、安全、邮件相关的必要内部对象

OpenFlare 历史对象如 `nodes`、`config_versions`、`proxy_routes`、`managed_domains`、`tls_certificates` 等，属于移除范围。

## 8. 文档维护原则

以下内容变化时，应同步更新本文档：

* 产品范围或系统边界变化
* 保留模块或删除模块变化
* 整体架构变化
* 工程目录结构目标变化
* 分层原则变化
