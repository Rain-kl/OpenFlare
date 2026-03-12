# ATSFlare 前端改造说明

## 1. 当前状态

前端改造已完成，`atsf_server/web` 的 Next.js 新版工程已经成为正式管理端基线。

当前结论：

* 旧版 CRA + Semantic UI 方案已退出基线
* 新版前端继续由 Go Server 以静态资源方式托管
* 前端改造过程中的阶段计划、迁移顺序与风险清单不再继续维护

---

## 2. 当前前端基线

新版管理端位于 `atsf_server/web`，当前基线为：

* Next.js 15 App Router
* React 19
* TypeScript
* Tailwind CSS 4
* TanStack Query
* React Hook Form + Zod
* Zustand（仅限轻量客户端状态）
* Vitest + Playwright

工程与运行方式：

* `next build` 后生成静态导出产物
* 构建后通过现有流程交由 `atsf_server` 托管
* 登录态继续兼容现有 Session/Cookie 体系

---

## 3. 当前结构约束

新版前端保持以下结构：

* `app/`：路由与布局
* `features/`：业务模块
* `components/`：复用组件
* `lib/`：请求、环境、工具、常量
* `store/`：少量跨页面 UI 状态
* `tests/`：前端测试

当前已覆盖的主要页面包括：

* 首页
* 反代规则
* 配置版本
* 节点管理
* 应用记录
* 网站
* 用户管理
* 设置
* 性能
* 登录、注册、重置密码、GitHub OAuth、关于页

---

## 4. 后续维护原则

后续不再按“前端改造专项”推进，而按正式前端工程进行维护：

* 新前端开发统一遵循 [docs/frontend-development-guidelines.md](./frontend-development-guidelines.md)
* 涉及项目级约束时，同时遵循 [docs/development-guidelines.md](./development-guidelines.md)
* 若后续再次调整前端架构、部署模式或技术基线，再新增专项计划文档
