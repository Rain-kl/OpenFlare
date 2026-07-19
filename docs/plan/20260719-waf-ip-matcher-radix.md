# WAF IP 匹配：Radix / lua-resty-ipmatcher

## 1. 目标与背景 (Goal & Context)

* **需求背景**：`ip_match` 对 IP 组 `ip_list` 做线性扫描，且每行强制 `ipv6_equal` + `ip_in_cidr`，大名单（订阅/自动规则可达万～十万级）时压测 RPS 约 65、OpenResty CPU 打满。
* **开发范围 (Scope)**：
  * **必做**：边缘热路径改为预处理索引 + O(W) 查询；IP 组快照加载时编译；节点内联 `ips`/`cidrs` 同样编译；Agent 镜像安装 `lua-resty-ipmatcher`；规格与 changelog。
  * **Out of Scope**：控制面协议变更、改 IP 组存储格式、Geo 匹配优化。

## 2. 设计与决策 (Design & Decisions)

* **选型**：OpenResty 使用 `resty.ipmatcher`（底层 Radix，支持 IP 与 CIDR 统一；可用 `match_bin(binary_remote_addr)`）。
* **编译时机**：
  * IP 组：`waf.ip_groups` 采纳新快照时为每组 `ip_list` 建 matcher，挂到 `group._matcher`。
  * 节点 `ips`/`cidrs`：首次匹配时合并列表建 matcher，用 weak 缓存或按 config 引用缓存。
* **回退**：`require("resty.ipmatcher")` 失败时用纯 Lua「exact set + 预解析 CIDR」回退（测试 / 未装 opm 的本地 OpenResty），避免回归到每行 IPv6 全解析。
* **不引入**：手写纯 Lua 十万节点 table 树作为生产主路径（内存与 GC 差）。

## 3. 具体修改文件清单 (Proposed Changes)

### 边缘 Agent 与 OpenResty

* #### [MODIFY] `docker/Dockerfile.agent`
  * `opm get api7/lua-resty-ipmatcher`（与 maxminddb 并列）。
* #### [MODIFY] `internal/apps/agent/nginx/waf_runtime.lua`
  * 编译/查询 helper；重写 `matches_ip_values`。
* #### [MODIFY] `internal/apps/agent/nginx/waf_ip_groups.lua`
  * 无需在刷新模块内编译；快照采纳后由 `waf.runtime` 惰性编译 `group._matcher`。
* #### [MODIFY] `internal/apps/agent/nginx/waf_runtime_spec.lua` / `waf_ip_groups_spec.lua`
  * 覆盖 exact/CIDR/IPv6/组 miss；大名单语义 smoke。
* #### [MODIFY] `docs/changelog/index.md`、相关设计/plan 备注

## 4. 验证计划 (Verification Plan)

* `go test ./internal/apps/agent/nginx/ -count=1`
* 重建 Agent 镜像后压测：三组大名单 miss 路径 CPU/RPS 对比。
