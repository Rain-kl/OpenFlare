# AGENTS.md

本文件是当前项目的 AI 接手入口。项目主线已从 OpenFlare 业务功能开发，切换为“裁剪现有工程并沉淀为可复用模板工程”。

本文件不承载详细设计、规范和计划。接手项目时，先按顺序阅读以下文档：

1. [docs/template-refactor-plan.md](./docs/template-refactor-plan.md)
   作用：理解本次模板工程改造的背景、目标边界、保留模块、移除范围、实施阶段、风险和验收标准。

2. [docs/design.md](./docs/design.md)
   作用：理解当前产品范围、系统边界、核心对象和整体架构。
   注意：如果其内容仍带有 OpenFlare 历史边界，应以模板化改造计划为当前主线，并优先推动设计文档收敛。

3. [docs/development-guidelines.md](./docs/development-guidelines.md)
   作用：理解当前开发规范，包括技术基线、分层约束、数据模型边界、API 约定、测试要求。

4. [docs/development-plan.md](./docs/development-plan.md)
   作用：理解当前开发阶段、实施顺序、阶段目标和验收标准。
   注意：如果其内容尚未切换到模板工程主线，应以 `docs/template-refactor-plan.md` 为准，并推动计划文档同步收敛。

5. [docs/frontend-development-guidelines.md](./docs/frontend-development-guidelines.md)
   作用：理解前端的技术选型、目录分层、组件规范、请求层、状态管理、样式和测试约束。

6. [docs/deployment.md](./docs/deployment.md)
   作用：理解当前的部署方式和联调步骤，确保开发过程中产出的功能能够成功部署和验证。

7. [docs/app-config.md](./docs/app-config.md)
   作用：理解系统启动时支持的环境变量和配置项说明，确保开发过程中新增或删除的配置项能够正确使用和文档化。


## 当前主线

当前默认主线不是继续扩展 OpenFlare 的节点、心跳、同步、版本分发、OpenResty 管理等业务，而是完成模板工程改造：

* 保留基础可复用能力：用户、版本升级、邮箱、文件上传、安全等模块
* 移除 OpenFlare 强业务耦合能力：Agent、节点、配置分发、代理规则、证书域名、观测分析等模块
* 裁剪重点集中在 `openflare_server`
* `openflare_agent` 最终应整体删除
* 同步推进工程规范化，服务端严格遵循 MVC 架构开发
* 同步推进目录结构规范化，逐步收敛到标准 Go 项目布局：`cmd/`、`internal/`、`pkg/`


## 执行要求

* 如果实现内容超出模板工程目标边界，先修改 `docs/template-refactor-plan.md` 和 `docs/design.md`，再继续编码。
* 如果 `docs/design.md`、`docs/development-plan.md` 与模板工程主线冲突，优先以 `docs/template-refactor-plan.md` 为当前改造依据，并补齐相关文档。
* 如果实现方式违反 `docs/development-guidelines.md`，应优先调整方案，而不是绕过规范。
* 如果任务涉及前端改造或管理端 UI，必须同时阅读 `docs/frontend-development-guidelines.md`。
* 如果任务涉及删除 OpenFlare 业务能力，接口与界面必须同步移除；应同时检查后端路由、模型、前端入口、导航、API client、类型定义、Swagger、部署文档和配置项，避免出现残留入口。
* 如果任务涉及新增模板能力，应优先复用现有通用基础设施，而不是沿用 OpenFlare 领域对象继续扩展。
* 服务端开发必须严格遵循 MVC：`controller/` 仅负责请求处理和响应封装，`service/` 负责业务逻辑与流程编排，`model/` 负责数据模型与持久化；不要把业务逻辑重新堆回 controller 或 middleware。
* 目录调整属于本次改造范围，后端目录应逐步迁移到 `cmd/`、`internal/handler`、`internal/service`、`internal/repository`、`internal/model`、`internal/dto`、`internal/middleware`、`internal/pkg`、`pkg/` 的规范结构。


## 文档维护要求

当以下内容发生变化时，应同步更新对应文档：

* 模板工程改造范围、阶段目标、保留/删除清单变化：更新 `docs/template-refactor-plan.md`
* 产品范围或系统边界变化：更新 `docs/design.md`
* 开发约束、代码规范、接口约定变化：更新 `docs/development-guidelines.md`
* 目录结构与工程组织方式变化：同步更新 `docs/template-refactor-plan.md`、`docs/development-guidelines.md`
* 阶段目标、顺序、验收标准变化：更新 `docs/development-plan.md`
* 前端目录分层、组件规范、样式体系、测试基线变化：更新 `docs/frontend-development-guidelines.md`
* 产品启动配置、部署方式或联调步骤变化：更新 `docs/deployment.md` 和 `README.md`
* 环境变量或配置项变化：更新 `docs/app-config.md`
