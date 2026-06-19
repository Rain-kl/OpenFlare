# 开发计划与 AI 接手

本分区用于存放正在进行的开发计划（Plan）以及 AI 代理之间的工作接手计划（Handover）。这能帮助不同的 AI 代理快速掌握当前项目状态、历史上下文与后续开发步骤。

## 计划模板

在创建具体的开发计划或接手文档时，请使用以下标准模板进行初始化：

1. **[实现计划模板](./implementation-plan-template.md)**：用于新功能开发或重大重构前的技术方案规划。
2. **[AI 接手计划模板](./handover-plan-template.md)**：用于在上下文截断、压缩或更换 AI 代理时，记录当前任务状态、已完成内容与下一步执行计划。

## 正在进行的计划

| 计划 | 说明 |
| --- | --- |
| [OpenFlare → Wavelet 后端迁移计划](./20260618-openflare-wavelet-backend-migration.md) | 将 `openflare-server` 后端迁移至 Wavelet 框架，保留 `/api/*` 路径，复用用户/认证等平台能力 |
| [OpenFlare 后端迁移 — AI 接手](./handover-openflare-backend-migration.md) | 后端迁移当前进度、任务队列、定时任务、goose 版本与下一步行动（阶段 5 收尾） |
| [OpenFlare → Wavelet 前端迁移计划](./20260618-openflare-wavelet-frontend-migration.md) | 将 `openflare-server/web` 业务 UI 按 Wavelet 设计风格重写，复用框架组件与 Admin 基建 |
| [OpenFlare 前端迁移 — AI 委派](./handover-openflare-frontend-migration.md) | 前端迁移任务队列与验收状态 |
| [前端路由验证](./verify-frontend-routes.md) · [Service 验证](./verify-frontend-services.md) · [UI 验证](./verify-frontend-ui.md) · [构建验证](./verify-frontend-build.md) | 多角度迁移验收报告 |
| [文档结构更新 — AI 接手](./handover-docs-restructure-update.md) | 重构后多智能体分析结论与文档批量更新记录 |
| [边缘运行时 Phase 3 任务拆解](./20260619-edge-phase3-tasks.md) | Batch 1 并行任务（duration / observability / heartbeat loop） |

## 使用建议

* **命名规范**：正在进行的开发计划建议命名为 `docs/plan/YYYYMMDD-[feature-name].md`，接手计划建议命名为 `docs/plan/handover-[task-name].md`。
* **物理隔离**：本目录下的计划文件只在开发周期内进行更新。当对应功能开发完毕并上线后，相应的计划文档应予以保留或归档，以供日后维护与新 AI 追溯历史决策。
* **禁止空文件**：请确保新创建的计划文档均基于对应的模板进行初始化填充。
