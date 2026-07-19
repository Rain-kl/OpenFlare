# 开发计划与 AI 接手

本分区用于存放正在进行的开发计划（Plan）以及 AI 代理之间的工作接手计划（Handover）。这能帮助不同的 AI 代理快速掌握当前项目状态、历史上下文与后续开发步骤。

## 计划模板

在创建具体的开发计划或接手文档时，请使用以下标准模板进行初始化：

1. **[实现计划模板](./implementation-plan-template.md)**：用于新功能开发或重大重构前的技术方案规划。
2. **[AI 接手计划模板](./handover-plan-template.md)**：用于在上下文截断、压缩或更换 AI 代理时，记录当前任务状态、已完成内容与下一步执行计划。

## 正在进行的计划

当前进行中的开发计划：

* [Zone 与域名资源重构](./20260712-zone-domain-refactor.md)：以 Zone 和正规化 Zone 域名替代托管域名及反代路由中的域名/证书冗余字段。
* [WAF 可编排规则](./20260713-waf-orchestration.md)：使用 React Flow 编辑 DAG 规则，发布时编译并由 OpenResty 纯内存执行。
* [边缘可观测与业务流量统计重构](./20260717-observability-redesign.md)：访问日志为业务唯一真相；Agent 只上报明细与主机读数；收敛「出站/已提供」双字段。
* [访问日志 cache_status 明细可见](./20260718-access-log-cache-status.md)：上报 `$upstream_cache_status`，明细展示命中/回源/未缓存三态。
* [边缘缓存默认 static 策略](./20260718-edge-cache-static-default.md)：开启缓存默认仅静态扩展名；存量 url→all。
* [访问日志 IP 明细 Tab](./20260719-access-log-ip-tab.md)：第三 Tab 按 IP 聚合列表（时间窗/流量/2xx 比例）；IP 情报迁入独立详情；日志详情仅请求字段。

## 已完成的计划

* [Pages 项目部署源与 GitHub Releases 自动更新 V2](./20260719-pages-source-sync-v2.md)：已完成 Remote URL / GitHub Release 来源、不可变部署、自动检查更新与安全回滚，并预留独立仓库构建 Provider 边界；生产环境验收边界见计划内验证记录。

## 使用建议

* **命名规范**：正在进行的开发计划建议命名为 `docs/plan/YYYYMMDD-[feature-name].md`，接手计划建议命名为 `docs/plan/handover-[task-name].md`。
* **物理隔离**：本目录下的计划文件只在开发周期内进行更新。当对应功能开发完毕并上线后，相应的计划文档应予以保留或归档，以供日后维护与新 AI 追溯历史决策。
* **禁止空文件**：请确保新创建的计划文档均基于对应的模板进行初始化填充。
