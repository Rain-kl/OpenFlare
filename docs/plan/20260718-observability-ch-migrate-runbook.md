# ClickHouse 观测表迁移与小时汇总回填（运维手册）

适用：M5 观测存储（`of_node_edge_health`、`of_access_log_hourly`、删旧表）及历史小时回填。

## 前提

* 控制面 `config.yaml` / 环境变量中 ClickHouse 已启用，账号可写 `openflare` 库。
* 备份策略已就绪（可选：对 `of_node_access_logs` 做快照）。
* **Agent 升级策略为销毁重建**；勿混跑旧 Agent（旧协议字段已从 Server 删除）。

## 1. 自动迁移（推荐）

进程启动时 `migrator.MigrateClickHouse()` 会按 goose 顺序执行：

| 版本 | 作用 |
| --- | --- |
| `202607180001` | access log 增加 `request_length` / `request_time_ms` |
| `202607180002` | 建 `of_node_edge_health`、`of_access_log_hourly`(+MV)；删 request_reports / openresty 吞吐表 |
| `202607180003` | 从明细 ANTI JOIN 回填近 90 天 `of_access_log_hourly` |

启动 API / all 模式一次即可：

```bash
# 示例：本地
./bin/openflare api
# 或
make run   # 以项目实际入口为准
```

查看 goose 版本表（ClickHouse）确认三版本均已应用。

## 2. 仅回填（迁移已执行、MV 创建前缺历史）

若只需重跑回填 SQL：

```bash
clickhouse-client --host 127.0.0.1 --port 9000 \
  --user default --password "$CLICKHOUSE_PASSWORD" \
  --database openflare \
  --multiquery < internal/db/migrator/goose/clickhouse/202607180003_backfill_access_log_hourly.sql
```

（goose 文件含 `+goose Up` 注释，若 client 报错可去掉注释行后执行 INSERT 主体。）

回填可重复：`ANTI JOIN` 跳过已有 `(node_id, hour, host)`。

## 3. 验收

```sql
-- 新表存在
SHOW TABLES FROM openflare LIKE 'of_node_edge_health';
SHOW TABLES FROM openflare LIKE 'of_access_log_hourly';

-- 旧表应不存在
SHOW TABLES FROM openflare LIKE 'of_node_request_reports';
SHOW TABLES FROM openflare LIKE 'of_node_obs_openresty';

-- 小时汇总有数据（有历史访问时）
SELECT count() FROM of_access_log_hourly;
SELECT min(hour), max(hour), sum(request_count) FROM of_access_log_hourly;
```

看板 24h 请求趋势应优先走 hourly；UV 卡片为整窗独立访客，**不等于**小时 UV 之和。

## 4. 本机执行记录

| 日期 | 环境 | 结果 |
| --- | --- | --- |
| 2026-07-18 | 开发机 | Docker daemon 未启动，未能 live 迁移；SQL 与 goose 文件已入库 |

运维在目标环境按 §1–§3 执行后更新本表。
