# ClickHouse P0–P3 修复计划

> 状态: 已完成（已合并主工作区，`make code-check` 通过）  
> 策略: 4 个互不干扰 worktree 并行，最后由主代理合并

## 任务拆分

| ID | Worktree 主题 | 范围 | 禁止改动 |
|----|---------------|------|----------|
| WT1 | P0 清理语义 C1 | cleanup maintenance / delete / tasks | chwriter、dashboard、DDL 新 MV |
| WT2 | 写路径 C2+H1+H2+H3 | chwriter、batchwriter、risk_control、model store 分层、status 指标 | goose 迁移、dashboard 读逻辑 |
| WT3 | 读路径 H4+H5 | 最新快照查询、metric/openresty 小时 MV + 读路径 | chwriter、cleanup |
| WT4 | P3 打磨 | 连接池/async_insert、traffic hourly TTL、UV 语义 | model store 分层、cleanup |

## 合并顺序

1. WT1 → 2. WT2 → 3. WT3 → 4. WT4  
（迁移文件时间戳已错开，changelog 由主代理统一写）

## 验收

各 worktree: 相关 `go test` + 可运行部分；合并后 `make code-check`。
