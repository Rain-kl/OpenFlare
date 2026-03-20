# GinNextTemplate 开发计划

本文档描述 `GinNextTemplate` 当前阶段的实施顺序、阶段目标和验收标准。

当前阶段主线不是继续扩展 OpenFlare，而是完成模板工程化改造。

## 1. 当前结论

* 项目已进入模板工程重构阶段
* 当前正式名称为 `GinNextTemplate`
* 当前优先级高于历史 OpenFlare 业务迭代
* 一切开发工作都应服务于模板工程边界收敛、目录规范化和可复用能力沉淀

## 2. 当前阶段划分

### 阶段一：文档与目标边界重定义

目标：

* 将产品名称统一为 `GinNextTemplate`
* 将文档主线从 OpenFlare 切换为模板工程
* 明确保留模块、删除模块、MVC 规则和目录结构目标

验收标准：

* `design`、`guidelines`、`plan`、`deployment`、`app-config`、`README` 全部切换到模板工程口径
* 文档不再把 Agent、节点、OpenResty、配置分发定义为系统主干

### 阶段二：模型与配置瘦身

目标：

* 收敛模板工程需要的数据模型和配置项

验收标准：

* OpenFlare 专属模型进入移除范围
* 配置项不再围绕 Agent、节点、OpenResty 和观测能力展开

### 阶段三：后端接口裁剪与 MVC 收敛

目标：

* 后端只保留模板保留模块
* 服务端开始向 `handler/service/repository/model/dto` 结构收敛

验收标准：

* OpenFlare 强业务接口完成裁剪
* Swagger 与接口保持一致
* 保留模块按新的分层方式组织

### 阶段四：前端同步裁剪

目标：

* 界面只保留模板工程相关功能

验收标准：

* 页面、导航、请求层和接口裁剪保持同步
* 不存在失效页面或失效请求

### 阶段五：目录结构重组

目标：

* 服务端目录迁移到标准 Go 工程布局

验收标准：

* 应用入口迁移到 `cmd/`
* 核心代码迁移到 `internal/`
* 公共能力按 `internal/pkg` 和 `pkg` 收敛

### 阶段六：移除 Agent 与部署残留

目标：

* 从仓库中彻底移除 Agent 相关内容

验收标准：

* `openflare_agent` 被删除
* 部署、配置、README、脚本中不再残留 Agent 主线

### 阶段七：回归验证与模板固化

目标：

* 确保模板工程可作为新项目起点稳定使用

验收标准：

* 服务端启动成功
* 前端构建成功
* 用户、邮箱、上传、安全、升级主链路可用
* 文档、代码、目录结构和配置项一致

## 3. 当前优先级

当前开发应优先关注：

1. 文档准确性
2. 模板工程边界收敛
3. 接口与界面同步裁剪
4. MVC 分层和目录结构规范化
5. 测试与回归补强

## 4. 变更准入原则

新需求进入实现前，按以下顺序判断：

1. 是否符合 [docs/design.md](./design.md) 的模板工程边界
2. 是否符合 [docs/template-refactor-plan.md](./template-refactor-plan.md) 的实施主线
3. 是否符合 [docs/development-guidelines.md](./development-guidelines.md) 的 MVC 与目录结构约束
4. 是否会造成接口与界面不同步
5. 是否需要同步修改部署、配置或 README 文档

如果答案包含“超出边界”或“破坏当前阶段主线”，必须先修改文档，再开始实现。

## 5. 当前验收标准

任意合入正式基线的改动，至少应满足：

* 不偏离 `GinNextTemplate` 的模板工程定位
* 不保留明显的 OpenFlare 历史残留入口
* 接口删除和界面删除保持同步
* 符合 MVC 分层与目录结构目标
* 有与风险相称的测试或联调验证
* 文档与代码保持一致

## 6. 后续维护方式

后续规划不再以“历史 OpenFlare 大版本阶段文档”方式维护，而按以下方式收敛：

* 产品边界变化：更新 `docs/design.md`
* 开发约束变化：更新 `docs/development-guidelines.md`
* 模板改造计划变化：更新 `docs/template-refactor-plan.md`
* 部署与配置变化：更新 `docs/deployment.md`、`docs/app-config.md` 和 `README.md`
