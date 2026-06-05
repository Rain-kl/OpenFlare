---
name: openflare-plan
description: OpenFlare 项目级技能：规定在开启新方案、新计划或进行任务交接时，必须将计划落库到 docs/plan 文件夹中并使用对应模板。
---

# OpenFlare Plan & Handover Skill

当你在 OpenFlare 项目中被要求“开启一个新的方案”、“制定开发计划”或者准备“任务交接（Handover）”时，你**必须**遵循本技能的工作流，将计划或方案落库到 `docs/plan/` 目录下。

## 执行工作流 (Workflow)

### 1. 确定计划类型
* **新特性/技术实现计划**：如果你要开发新功能或进行重大重构，你需要创建**实现计划**。
* **AI 任务交接计划**：如果当前任务尚未完成但需要记录进度留作以后/其他 AI 代理接手，你需要创建**交接计划**。

### 2. 读取对应模板
在使用创建计划文档前，必须读取对应的模板内容，并严格按照模板的骨架进行填充：
* **实现计划模板**：`docs/plan/implementation-plan-template.md`
* **接手计划模板**：`docs/plan/handover-plan-template.md`

### 3. 落库与命名规范
在 `docs/plan/` 目录下创建新的 Markdown 文件进行保存：
* **实现计划**命名格式：`docs/plan/YYYYMMDD-[feature-name].md` (例如：`20260605-uptime-kuma-sync.md`)
* **接手计划**命名格式：`docs/plan/handover-[task-name].md` (例如：`handover-waf-ip-group.md`)

### 4. 隔离约束 (极其重要)
`docs/plan/` 目录下的文档**仅限内部开发和 AI 代理同步使用**。
* **绝对禁止**将新创建的 plan 文档加入到 `docs/config.ts` 的 `nav` 或 `sidebar` 导航条中。
* **绝对禁止**通过任何方式将其暴露给 VitePress 对外渲染。

## 后续动作
落库完成后，向用户报告计划已生成在 `docs/plan/` 目录下，并列出文档的核心要点或待决策项（如有），等待用户 Review 或批准后即可推进下一步。
