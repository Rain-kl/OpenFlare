# 自动 IP 组规则语法

按时间窗口聚合访问日志并执行 Expr 布尔表达式，将命中异常特征的公网 IP 自动写入 WAF **IP 匹配**组。

## 1. 配置结构与过滤口径

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `lookback` | string | 回溯窗口（Go Duration 语法，如 `30m`、`1h`），默认 `1h`，上限 `30` 天（兼容旧整数字段 `lookback_minutes`） |
| `rules` | array | 规则列表；任一条规则返回 `true` 即收录该 IP |
| `rules[].name` | string | 用于控制台展示与错误提示的规则名称 |
| `rules[].expr` | string | 返回布尔值的 Expr 表达式 |

评估与 **测试规则** 均按规范化 `remote_addr` 聚合，并自动跳过私网 IPv4、IPv6 ULA、回环、链路本地、未指定、组播及广播地址（含 IPv4-mapped IPv6）；定时任务会自动清理不符合该口径的历史记录。

## 2. 可用变量与函数

| 标识符 | 类型 | 说明 |
| :--- | :--- | :--- |
| `ip` | string | 当前评估的客户端 IP |
| `request_count` | number | 窗口内总请求数 |
| `status_404_count` | number | 返回 HTTP `404` 的请求数 |
| `status_404_ratio` | number | `404` 请求占比（`0` 到 `1`，如 `0.8` 表示 80%） |
| `ip_host_count` | number | `Host` 头为纯 IP 字面量（如 `203.0.113.10`）的请求数 |
| `ip_host_ratio` | number | 纯 IP `Host` 请求占比（`0` 到 `1`） |
| `client_error_count` | number | `4xx` 响应总数 |
| `server_error_count` | number | `5xx` 响应总数 |
| `last_seen_unix` | number | 窗口内最近一次请求的 Unix 秒级时间戳 |
| `StatusCount(code)` | number | 指定状态码（`403`）或状态分类（`"4xx"`）计数 |
| `StatusRatio(code)` | number | 指定状态码（`502`）或状态分类（`"5xx"`）占比（`0` 到 `1`） |

## 3. 运算符与内置预设

| 运算符 | 含义 | 示例 |
| :--- | :--- | :--- |
| `>`、`>=`、`<`、`<=` | 数值比较 | `request_count > 100` |
| `==`、`!=` | 等值比较 | `ip != "203.0.113.10"` |
| `&&`、`||`、`!` | 逻辑与、或、非 | `request_count > 100 && StatusRatio(404) >= 0.8` |
| `in`、`not in` | 列表成员判断 | `ip not in ["203.0.113.10", "198.51.100.20"]` |
| `()` | 优先级分组 | `(client_error_count > 80 && request_count > 100) || server_error_count > 30` |

编辑器内置两条常用预设：

| 预设名称 | 表达式 |
| :--- | :--- |
| 单 IP 高频 404 扫描 | `request_count > 100 && StatusRatio(404) >= 0.8` |
| 单 IP 直连 IP 访问 | `ip_host_count > 50 && ip_host_ratio > 0.5` |

组合示例：

```json
{
  "lookback": "2h",
  "rules": [
    {
      "name": "Single-IP high-frequency 404 scanning",
      "expr": "request_count > 100 && StatusRatio(404) >= 0.8"
    },
    {
      "name": "High error ratio excluding trusted IPs",
      "expr": "ip not in [\"203.0.113.10\", \"198.51.100.20\"] && request_count > 100 && (StatusRatio(\"4xx\") >= 0.8 || StatusRatio(\"5xx\") >= 0.3)"
    }
  ]
}
```

## 4. 使用建议

- 上线前先用较短 `lookback` 与宽松阈值点击 **测试规则** 预览命中名单，避免误封。
- 定时刷新会整体覆盖自动 IP 组列表；长期白名单或永久封禁请放在手动 IP 组中并于 WAF 规则流联合引用。
