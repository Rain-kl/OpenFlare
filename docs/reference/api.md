# API 约定

你会学到：OpenFlare 管理端 API 与 Agent API 的响应结构、路径约定、鉴权方式和 Swagger 入口。

OpenFlare 的管理端 API 与 Agent API 都使用 JSON。

## 响应结构

成功与失败都应返回清晰的 `message`：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

## 路径约定

| 类型 | 约定 |
| --- | --- |
| 管理端 API | 由管理端 Session 鉴权 |
| Agent API | 固定放在 `/api/agent/*` |
| 只读接口 | 使用 `GET` |
| 变更类接口 | 使用 `POST` |

## WAF IP 组接口

管理端 WAF IP 组接口统一要求管理端 Session 鉴权：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/waf/ip-groups` | 查询 IP 组列表 |
| `GET` | `/api/waf/ip-groups/:id` | 查询单个 IP 组 |
| `POST` | `/api/waf/ip-groups` | 创建 IP 组 |
| `POST` | `/api/waf/ip-groups/test` | 测试自动 IP 组 Expr 规则，不保存配置，返回当前日志窗口内命中的 IP 列表 |
| `POST` | `/api/waf/ip-groups/:id/update` | 更新 IP 组 |
| `POST` | `/api/waf/ip-groups/:id/delete` | 删除 IP 组；已被规则组引用时会拒绝 |
| `POST` | `/api/waf/ip-groups/:id/sync` | 立即同步订阅型 IP 组或立即执行自动型 IP 组 |

IP 组 `type` 支持 `manual`、`automatic`、`subscription`。自动型 IP 组的 `auto_config` 是 JSON 对象，当前支持：

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "单 IP 404 高频扫描",
      "expr": "request_count > 100 && status_404_ratio >= 0.8"
    },
    {
      "name": "单 IP 直连访问异常",
      "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
    }
  ]
}
```

自动规则使用 Expr 语法，表达式必须返回布尔值。规则按单个 IP 的请求日志聚合指标计算，可用字段包括 `ip`、`request_count`、`status_404_count`、`status_404_ratio`、`ip_host_count`、`ip_host_ratio`、`client_error_count`、`server_error_count`、`last_seen_unix`。完整语法和字段含义见 [WAF 自动 IP 组规则语法](../guide/waf-ip-group-expr.md)。订阅格式支持 `text` 与 `json`：文本格式按行解析 IP/IP 段并忽略空行和 `#` 开头的注释；JSON 格式可通过映射规则选择数组，默认读取根数组。

## 鉴权

管理端继续复用现有登录、角色与 Session。

Agent 正式请求统一使用节点专属 `agent_token`，首次接入可使用全局 `discovery_token`。Agent 请求头固定为：

```http
X-Agent-Token: <token>
```

### Agent WAF IP 组同步

Agent 心跳 payload 可携带本地 WAF IP 组 checksum：

```json
{
  "waf_ip_group_checksums": {
    "1": "sha256..."
  }
}
```

Server 会根据当前激活版本引用的 IP 组 ID 对比 checksum，并在心跳响应顶层返回差异组：

```json
{
  "waf_ip_groups": [
    {
      "id": 1,
      "name": "自动黑名单",
      "type": "automatic",
      "enabled": true,
      "ip_list": ["203.0.113.10"],
      "checksum": "sha256..."
    }
  ]
}
```

Agent 也可以在应用新版本后主动请求差异同步：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/agent/waf/ip-groups/sync` | 根据 Agent 上报的 `ids` 与 `checksums` 返回不一致的 IP 组 |

当 Server 侧 IP 组更新时，已连接的 Agent WebSocket 会收到 `type = "waf_ip_groups"` 的消息，payload 为发生变化的 IP 组数组。Agent 应只更新收到的组，不要求 Server 每次下发全部 IP 组。

日志中不得打印完整 Token。

## Swagger

登录管理端后可访问：

```text
/swagger/index.html
```

Swagger 文件位于 `openflare_server/docs`，由 `swag init` 生成。
