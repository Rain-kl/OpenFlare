# 攻击防护规则目录与新增类别

控制台使用 Edge `attack-rule-catalog` 命令生成的元数据展示检测规则。目录与运行时共用规则分类、说明和等级映射，Edge 测试比较生成结果与前端静态资产。修改规则后运行 `cargo run --locked --manifest-path edge/pingora/Cargo.toml -- attack-rule-catalog > frontend/lib/openflare/attack-rule-catalog.generated.json` 更新目录。

CSRF 使用 `11151`、`11152` 两条 OpenFlare 规则。Edge 将浏览器请求的来源检查结果传入 ModSecurity 事务，沿用流程图分支与日志协议。畸形 HTTP 选择 CRS 920 和 921 中与协议格式相关的规则，避免默认启用客户端标识、允许的媒体类型等业务策略。已有规则组缺少新类别即禁用。
