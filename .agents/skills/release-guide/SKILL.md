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
3. 将内部实现描述改写为用户可理解的变更。
4. 每条使用完整中文句子。
5. 尽量说明“修复/优化了什么”以及“带来的效果”。
6. 不要编造 commit log 中没有的信息。
7. 不要加入 token、密钥、私有地址等敏感信息。
8. 如果某个分类没有内容，可以省略。

固定使用以下分类：

```text
### 🛠 修复
### ⚡️ 优化与改进
### 💄 其他/体验
```

分类规则：

- Bug、异常行为、错误逻辑：放入 ### 🛠 修复
- 性能、稳定性、接口、架构、兼容性：放入 ### ⚡️ 优化与改进
- 日志、文案、UI、文档、开发体验：放入 ### 💄 其他/体验

示例:

```
chore(release): v3.3.0

### 🛠 修复
- 修复了通过 MCP 接口操作时笔记库范围限制未正确生效的问题。
- 修复了 MCP 接口返回数据格式不一致的问题。
- 修复了 WebSocket 客户端异常断开后僵尸连接未及时清理的问题。

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
