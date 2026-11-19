# Pingora 第三阶段策略契约

Pingora 第三阶段的边界是行为对齐，性能参数保持默认。主控仍从同一业务快照派生 OpenResty 配置与 Pingora payload，避免两条线使用不同的业务输入。契约 v2 将 `policy-runtime-v1` 声明为必需能力，路由内包含 Basic Auth、站点认证、独立 PoW、合并后的限流结果和 Pages 部署参数。限流合并沿用 OpenResty 的语义：路由 `-1` 显式关闭，空值或 `0` 继承全局默认，正数覆盖默认值。

Rust 侧优先实现可精确验证的纯策略层：站点认证使用与 Lua 一致的 `expiry.hmac_sha1_hex` Cookie、SHA-256 凭据摘要和 `__Host-openflare_site_auth` 剥离规则；Basic Auth 使用常量时间比较并返回相同 `WWW-Authenticate`；PoW 先完成路径、IP/CIDR/UA 与路径规则判定；限流按 OpenResty 的 burst 公式实现令牌桶；Pages 使用固定 state 目录下的 project/current 布局并禁止路径逃逸。

Pingora 请求生命周期用 `request_filter` 执行站点认证、Basic Auth、PoW 判定和请求限流，通过后继续原有代理路径。Pages 静态文件在此阶段直接返回；API 代理、PoW 挑战签发/验证、WAF 图执行和 GeoIP 尚未完成，后续实现继续复用本契约，不再回退到配置文件或控制面旁路。
