# 攻击防护逐条规则设置

`of_attack_groups.rule_overrides` 按规则 ID 存储 `off`、`enabled` 和 `observe`。目录状态由类别等级与逐条设置共同计算，缺少规则 ID 时沿用类别。更新接口省略 `rule_overrides` 时保留已有逐条设置，空对象清空设置。快照只在存在逐条设置时携带该字段。旧版 Edge 会拒绝含该字段的配置并继续使用上一份健康配置，启用逐条设置前须升级 Edge。

规则 ID 的校验表从 Edge 生成的目录派生。目录更新后运行 `node scripts/generate-attack-rule-ids.mjs`。Edge 编译检测器时将逐条设置纳入复用键。仅观察规则进入原有记录分支，开启规则由原有攻击防护节点的 `false` 分支处理。单独启用任一 CSRF 规则也会初始化来源检测上下文。
