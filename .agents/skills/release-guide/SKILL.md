---
name: "release-guide"
description: "Wavelet 项目专用：根据自上一个正式版本 Tag 以来的提交记录，整理生成规范的 Version Bump Commit Message，用于触发自动双语 Release。"
---

# Release Commit Message Guide

## 目标

当用户准备发布 Wavelet 新版本时，本 Skill 负责：

1. 根据上一正式版本 Tag 以来的提交，整理面向用户的发版说明；
2. 新建 **独立的** `chore(release): vX.Y.Z` 提交（可附带将 `docs/changelog` 从 `[unreleased]` 落版）。

## 硬性约束（禁止改写历史）

- **禁止** `git commit --amend` 修改任何**已经 push 到远端**的提交。
- **禁止** 为了发版去改写已有功能/修复提交的 message 或内容。
- **禁止** 发版流程中的 force-push（除非用户明确要求且知晓后果）。
- 发版提交必须是 **新增 commit**：在当前 `HEAD` 之上 `git commit` 一次。
- 默认 **不要 push、不要打 tag**；生成并完成本地 release commit 后，把后续 `push` / `git tag` 命令交给用户确认执行。

## 生成提交信息

将原始 commit log 整理为面向 Release 的更新说明。

要求：

1. 合并重复或相近提交。
2. 删除无意义提交，例如格式化、临时调试、无关重构。
3. 将内部实现描述改写为用户可理解的变更, 说明“修复/优化了什么”以及“带来的效果”。
4. 不要写技术细节：只描述用户可感知的行为与效果，禁止内部实现描述，例如字段名/表名/SQL（`node_id = ''`）、框架或库名称（shadcn、GORM、OpenResty）、配置或协议细节（RFC3339、ClickHouse/PostgreSQL 差异）、代码机制（`proxy_intercept_errors`、Lua 过滤器、雪花 ID）。数据库名称仅在说明受影响用户范围时使用（如「PostgreSQL 日志库下无数据」）。
5. 如果某个分类没有内容，则省略。

固定使用以下分类：

```text
### ✨ 新功能
### 🛠 修复
### ⚡️ 优化与改进
### 💄 其他/体验
```

分类规则：

- 新功能、新能力、新配置、新任务：放入 ### ✨ 新功能
- Bug、异常行为、错误逻辑：放入 ### 🛠 修复
- 性能、稳定性、接口、架构、兼容性：放入 ### ⚡️ 优化与改进
- 日志、文案、UI、文档、开发体验：放入 ### 💄 其他/体验

「修复/优化」与「新增」的判定（关键）：

- **判定标准是“该功能在上一正式版本中是否已存在”**：
  - 已存在 → 本次对其 bug 的修正可计入「🛠 修复」，对其行为/性能的改进可计入「⚡️ 优化与改进」；
  - 不存在（本版本新增）→ 该功能的一切内容——包括开发过程中修的 bug、做的性能优化、补的索引——都只属于新功能开发的一部分，不应该在发布说明中提及。
- 禁止把新功能的开发期修复/优化写进「修复」或「优化」：新功能此前版本没有，谈不上“修复/优化了旧行为”。

示例:

```
chore(release): v3.3.0

### ✨ 新功能
- 新增笔记库快照备份功能，支持定时备份与手动一键恢复（仅说明新增的功能, 禁止提及新功能开发时期的优化修复等内容）。

### 🛠 修复
- 修复首页「来源分布」卡片在 PostgreSQL/SQLite 日志库下无数据的问题。
- 修复源站错误页「仅针对 GET 请求」未真正透传非 GET 响应的问题：POST/PUT 等非 GET 请求现可完整看到源站原始报错内容。

### ⚡️ 优化与改进
- 优化了 WebGUI 登录机制，引入设备令牌自动轮转，减少因 IP 变化产生的冗余令牌。

### 💄 其他/体验
- 优化了 WebSocket 错误日志，增加请求路径信息，方便问题排查。
```

## 提交步骤

1. 确认工作区干净，且 `HEAD` 与将要发布的代码一致（通常已与 `origin/main` 对齐或仅含未 push 的合法新提交）。
2. 将 `docs/changelog/index.md` 中 `[unreleased]` 落版为 `[vX.Y.Z] - YYYY-MM-DD`（按需整理条目）。
3. **新建** release 提交（不要 amend）：

```bash
git add docs/changelog/index.md   # 及其他发版所需文件
git commit -m "$(cat <<'EOF'
chore(release): vX.Y.Z

### 🛠 修复
- ...

### ⚡️ 优化与改进
- ...

### 💄 其他/体验
- ...
EOF
)"
```

4. 向用户展示完整 commit message，并说明后续可由用户执行：

```bash
git push origin main
git tag vX.Y.Z
git push origin vX.Y.Z
```

（打 tag 后由 CI 创建双语 Release。）

## 任务结束条件

本地已存在 **新的** `chore(release): vX.Y.Z` 提交，且**未**改写任何已 push 提交、**未**擅自 push/tag。
