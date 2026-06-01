# WAF 自动 IP 组规则语法

自动 IP 组用于从请求日志中按单个客户端 IP 聚合指标，再用 Expr 表达式判断是否把该 IP 加入组内名单。自动 IP 组可以被 WAF 规则组的 IP 黑名单或白名单引用；发布配置时，Server 只把 IP 组引用 ID 写入 `waf_config.json`，IP 组成员由 Agent 独立同步到本地运行时文件。

## 配置结构

自动 IP 组的配置是一个 JSON 对象：

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "单 IP 404 高频扫描",
      "expr": "request_count > 100 && status_404_ratio >= 0.8"
    }
  ]
}
```

字段说明：

| 字段 | 类型 | 作用 |
| --- | --- | --- |
| `lookback_minutes` | number | 每次执行时回看多少分钟内的请求日志。未填写时默认 60 分钟，最小 5 分钟，最大 43200 分钟。 |
| `rules` | array | 自动规则列表。任意一条规则命中时，该 IP 会进入自动 IP 组名单。 |
| `rules[].name` | string | 规则名称，只用于界面展示和错误提示。 |
| `rules[].expr` | string | Expr 表达式，必须返回布尔值。 |

## 执行口径

自动规则不是逐条请求判断，而是先按单个客户端 IP 聚合：

1. Server 读取最近 `lookback_minutes` 分钟内的请求日志。
2. 按 `remote_addr` 归一化后的 IP 分组。
3. 为每个 IP 计算请求数、404 数、直连 IP Host 次数等指标。
4. 逐个 IP 执行 `rules[].expr`。
5. 只要某个 IP 命中任意规则，就写入该自动 IP 组的 `IP / IP 段` 列表。

Host 是否为“通过 IP 访问”按请求日志中的 `Host` 字段判断：如果 Host 是 IPv4 或 IPv6 字面量，例如 `203.0.113.10`、`[2001:db8::10]`、`203.0.113.10:443`，就计入 `ip_host_count`。

## 可用关键字

表达式中可以直接使用以下字段：

| 关键字 | 类型 | 作用 |
| --- | --- | --- |
| `ip` | string | 当前正在判断的客户端 IP。 |
| `request_count` | number | 当前 IP 在回看窗口内的总请求数。 |
| `status_404_count` | number | 当前 IP 在回看窗口内返回 404 的请求数。 |
| `status_404_ratio` | number | 404 请求占比，计算方式为 `status_404_count / request_count`。 |
| `ip_host_count` | number | 当前 IP 通过 IP 地址作为 Host 访问的请求数。 |
| `ip_host_ratio` | number | 通过 IP 地址访问的占比，计算方式为 `ip_host_count / request_count`。 |
| `client_error_count` | number | 当前 IP 返回 4xx 状态码的请求数。 |
| `server_error_count` | number | 当前 IP 返回 5xx 状态码的请求数。 |
| `last_seen_unix` | number | 当前 IP 在回看窗口内最后一次请求的 Unix 秒级时间戳。 |

比例字段都是 `0` 到 `1` 之间的小数。80% 应写成 `0.8`，50% 应写成 `0.5`。

## Expr 常用写法

自动 IP 组使用 Expr 语法，当前表达式必须返回布尔值。

常用运算符：

| 写法 | 作用 | 示例 |
| --- | --- | --- |
| `>`、`>=`、`<`、`<=` | 数值比较 | `request_count > 100` |
| `==`、`!=` | 相等或不相等 | `ip != "127.0.0.1"` |
| `&&` | 并且 | `request_count > 100 && status_404_ratio >= 0.8` |
| `||` | 或者 | `status_404_ratio >= 0.8 || server_error_count > 20` |
| `!` | 取反 | `!(ip == "127.0.0.1")` |
| `in` | 判断值是否在列表中 | `ip in ["203.0.113.10", "198.51.100.20"]` |
| `not in` | 判断值是否不在列表中 | `ip not in ["127.0.0.1"]` |
| `()` | 分组控制优先级 | `(request_count > 100 && status_404_ratio >= 0.8) || server_error_count > 50` |

## 内置预设

管理端内置两个预设规则，可以直接添加后再按需调整：

```json
{
  "name": "单 IP 404 高频扫描",
  "expr": "request_count > 100 && status_404_ratio >= 0.8"
}
```

含义：单个 IP 在回看窗口内请求数大于 100，并且 404 状态码占比不低于 80%。

```json
{
  "name": "单 IP 直连访问异常",
  "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
}
```

含义：单个 IP 通过 IP 地址作为 Host 访问的次数大于 50，并且这种访问占比大于 50%。

## 示例

高频 404 扫描：

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "高频 404 扫描",
      "expr": "request_count > 100 && status_404_ratio >= 0.8"
    }
  ]
}
```

IP 直连访问异常：

```json
{
  "lookback_minutes": 30,
  "rules": [
    {
      "name": "IP 直连访问异常",
      "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
    }
  ]
}
```

同时捕获高 4xx 与高 5xx：

```json
{
  "lookback_minutes": 120,
  "rules": [
    {
      "name": "异常错误率",
      "expr": "(client_error_count > 80 && request_count > 100) || server_error_count > 30"
    }
  ]
}
```

排除可信 IP：

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "排除可信 IP 的 404 扫描",
      "expr": "ip not in [\"203.0.113.10\", \"198.51.100.20\"] && request_count > 100 && status_404_ratio >= 0.8"
    }
  ]
}
```

## 使用建议

先用较短的回看窗口和较高阈值观察命中结果，再逐步调整阈值。管理端 IP 组页面支持在保存前点击 **测试规则**，直接查看当前回看窗口内命中的 IP；自动 IP 组真正执行后会覆盖该组的 IP 列表。如果要长期保留某些地址，建议放入手动 IP 组，并在 WAF 规则组中同时引用手动组和自动组。

自动 IP 组更新后不需要重新发布配置版本。在线 Agent 会通过 WebSocket 收到变更 IP 组并更新本地 `waf_ip_groups.json`；WebSocket 不可用时，Agent 会在下一次心跳中上报本地 IP 组 checksum，Server 只返回 checksum 不一致的 IP 组。
