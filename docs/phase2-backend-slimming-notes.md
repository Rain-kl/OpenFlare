# 阶段二后端瘦身说明

本文件补充说明“阶段二：后端模型与配置瘦身”当前已经落地的收敛点，作为 `docs/template-refactor-plan.md` 的阶段性执行记录。

## 当前已完成

* 数据库核心注册模型已收敛为 `users`、`files`、`options`。
* 数据库 schema 版本已提升到 `v4`，现阶段校验只要求模板工程保留的核心表和 schema 元数据表。
* 服务端启动帮助信息与默认 SQLite 文件名已切换到 `GinNextTemplate`。
* `Option` 默认初始化项已移除 Agent、节点、OpenResty、GeoIP、观测相关配置。
* `Option` 更新接口已禁止继续写入上述已移除配置键，避免旧业务配置回流。

## 当前策略

* 历史 OpenFlare 表先按“兼容残留、停止扩展”处理，不在阶段二直接做物理删除。
* 历史 OpenFlare 常量和部分代码引用暂时保留，用于支撑后续阶段逐步删除接口、服务和界面。
* 后续阶段三、阶段四需要继续同步删除对应接口、Swagger、前端页面、导航、API client 和类型定义。

## 下一步建议

* 继续清理 `controller/service/router` 中仍然面向节点、Agent、OpenResty、配置分发、观测分析的接口和业务入口。
* 同步更新前端菜单、页面与请求层，确保接口与界面同批次收敛。
* 在目录迁移开始前，把保留模块先按 `handler/service/repository/model/dto` 的职责边界整理清楚。
