# AGENTS.md

本文件是 ATSFlare 的 AI 接手入口，不承载详细设计、规范和计划。接手项目时，先按顺序阅读以下文档：

1. [docs/design.md](./docs/design.md)
   作用：理解当前 MVP 的产品范围、系统边界、核心对象和整体架构。

2. [docs/development-guidelines.md](./docs/development-guidelines.md)
   作用：理解当前开发规范，包括技术基线、分层约束、数据模型边界、API 约定、Agent 约束、测试要求。

3. [docs/development-plan.md](./docs/development-plan.md)
   作用：理解当前开发阶段、实施顺序、阶段目标和验收标准。

3. [docs/deployment.md](./docs/deployment.md)
   作用：理解当前的部署方式和联调步骤，确保开发过程中产出的功能能够成功部署和验证。

4. [docs/app-config.md](./docs/app-config.md)
   作用：系统启动时支持的环境变量和配置项说明，确保开发过程中新增的配置项能够正确使用和文档化。


## 执行要求

* 如果实现内容超出 `docs/design.md` 的范围，先修改设计文档，再继续编码。
* 如果实现方式违反 `docs/development-guidelines.md`，应优先调整方案，而不是绕过规范。
* 如果需求与当前开发阶段冲突，优先遵守 `docs/development-plan.md` 的阶段顺序。


## 文档维护要求

当以下内容发生变化时，应同步更新对应文档：

* 产品启动配置部署方式发生变化时: 更新 `docs/deployment.md`和 `README.md`
* 产品范围或系统边界变化：更新 `docs/design.md`
* 开发约束、代码规范、接口约定变化：更新 `docs/development-guidelines.md`
* 阶段目标、顺序、验收标准变化：更新 `docs/development-plan.md`
* 环境变量或配置项变化：更新 `docs/app-config.md`
