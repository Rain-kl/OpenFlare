# 发布模型

你会学到：OpenFlare 为什么以完整配置版本为发布单位，发布、激活、Agent 应用和回滚分别如何工作。

OpenFlare 的发布模型以完整配置版本为中心，而不是在线修改节点配置。

标准链路：

```text
修改规则 -> 预览 / 查看 diff -> 发布 -> 生成完整配置版本 -> 激活版本 -> Agent 拉取 -> 本地应用 -> 上报结果
```

## 发布规则

Server 发布时必须：

1. 读取全部启用的 `proxy_routes`。
2. 读取 Server 侧 OpenResty 主配置、性能参数、缓存参数和必要 Lua 资源。
3. 读取域名与证书绑定关系。
4. 读取 WAF 全局规则组、自定义规则组、IP 组引用与网站绑定关系。
5. 保留 WAF 规则组引用的 IP 组 ID，渲染完整 OpenResty 配置与 WAF 运行时配置；IP 组成员不进入发布版本。
6. 计算 `checksum`。
7. 写入 `config_versions`。
8. 切换激活版本。
9. 让 Agent 在后续 heartbeat 中发现并应用。

版本号格式固定为 `YYYYMMDD-NNN`。

## 预览与发布

预览和 diff 是只读能力，不产生发布记录。

发布会生成新的完整配置版本。版本必须包含足够信息，让未来回滚时可以基于历史快照重新应用，而不依赖当前可变配置。

## 激活版本

全局同时只能有一个激活版本。当前不做按节点分组的差异化版本。

Agent 通过 heartbeat 获取激活版本摘要；当远端版本或 checksum 与本地状态不一致时，Agent 才进入同步流程。当 Agent WS 连接升级开启且连接可用时，Server 在发布或激活版本成功后会广播最新激活版本摘要，Agent 收到后复用普通同步流程立即拉取并应用配置。WS 不可用时仍按 HTTP heartbeat 间隔发现变更。

## 不可变历史

历史版本不可变。回滚不是修改旧版本，而是重新激活旧版本。

这样做的结果是：

* 每个版本都可以追溯。
* 回滚链路与普通发布应用链路一致。
* Agent 不需要理解“反向 patch”，只需要应用一个目标版本。

## Agent 应用策略

Agent 发现新版本后会：

1. 拉取目标版本详情。
2. 备份旧文件。
3. 写入主配置、路由配置、证书、必要 Lua 资源与 WAF/PoW 运行时配置。
4. 执行 OpenResty 配置校验。
5. reload；如果运行时未启动，则尝试用当前配置启动 OpenResty。
6. 上报成功、警告或失败。

如果新配置激活失败，Agent 必须尝试恢复运行；回滚成功时上报警告。若本地没有历史主配置可回滚，Agent 会写入内置安全兜底配置并尝试拉起 OpenResty：该配置对外只监听 `80` 端口，不包含任何用户路由，统一返回 `503 Service Unavailable` 与 `OpenFlare: No Valid Configuration`，同时保留本地 `stub_status` 健康检查入口。兜底启动成功时仍阻断失败目标版本并上报警告；存在历史主配置但回滚后仍无法恢复运行时上报失败。

某个目标 `version + checksum` 一旦应用失败并回退，Agent 会在本地状态中阻断该目标重复应用。只有远端激活版本或 checksum 发生变化，才允许再次尝试。

## 设计约束

* 发布必须读取全部启用的网站配置，而不是只渲染本次修改对象。
* 回滚通过重新激活旧版本实现，不修改历史版本。
* Agent API 固定使用节点专属 `agent_token`，首次接入可使用 `discovery_token`。
* Server 不提供远程 shell 或任意命令执行入口。
* 配置版本必须保存完整快照、渲染结果和 `checksum`。
* WAF 规则组、IP 组引用 ID 和网站绑定关系必须随完整配置版本进入快照与 checksum；IP 组成员由 Agent 独立按 checksum 差异同步，不受版本回滚影响。

## WAF IP 组运行时同步

WAF IP 组成员不纳入配置版本。发布版本只包含规则组直接 IP 与 `ip_whitelist_group_ids` / `ip_blacklist_group_ids`。Agent 应用版本后会从渲染出的 `waf_config.json` 中提取引用 ID，并向 Server 请求缺失或 checksum 不一致的 IP 组数据。

Agent 后续心跳会携带本地 IP 组 checksum。Server 根据当前激活版本引用的 IP 组 ID 对比 checksum，只返回差异组，避免每次心跳传输全部 IP 组。Server 在手动更新、订阅同步或自动规则执行后，会通过 Agent WebSocket 广播发生变化的 IP 组；WS 不可用时，下一次 HTTP heartbeat 仍会按 checksum 差异补齐。
