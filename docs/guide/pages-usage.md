# Pages 静态托管使用

你会学到：如何通过本地上传、Remote URL 或公开 GitHub Release asset 部署预构建静态站点，配置 SPA Fallback 与 API 反向代理，并安全地检查更新、自动发布和回滚。

---

## 核心机制与页面结构

OpenFlare Pages 受 Cloudflare Pages 的 Direct Upload 与部署历史交互启发，但当前处理的是**预构建产物**，不是仓库源码构建。项目详情按“当前生产部署 → 部署源 → 部署历史”组织：来源配置可以变化，已经创建的 deployment 保持不可变。

```text
本地上传 ─> 统一校验 / upload.Ingest ─> 新 candidate ─> 管理员显式激活 ─┐
Remote URL ── Server 受限下载 ────────┐                                │
GitHub Release asset ─ Server 解析 ───┴─> create/load deployment ─────┤
                                      └─> source sync 原子激活 ────────┘
                                                                        |
                                                                        v
                                                        Agent 按项目 latest 拉取
                                                                        |
                                                                        v
                                                        OpenResty 本地静态服务
```

外部 URL、GitHub 元数据和自动检查都只由 Server 处理。Agent 只从控制面拉取当前激活的部署包，不接收外部来源凭据，也不执行 `git clone`、依赖安装或构建命令。

## 第一步：创建项目

1. 登录管理端，进入 **「Pages」**，点击 **「创建项目」**。
2. 填写项目名称与唯一 Slug。
3. 配置内容入口：
   * **入口文件名**：默认 `index.html`。
   * **静态资源根路径（RootDir）**：产物位于 `dist/` 等子目录时填写该相对路径；产物就在归档根目录时留空。
4. 按需设置 SPA Fallback 与 API 代理。RootDir 和入口文件是项目级配置，会统一应用于所有来源。

## 第二步：选择部署源

### 1. 手动上传

不配置持久来源时，项目保持手动模式。点击 **「上传部署包」** 选择预构建归档；上传成功会创建一条候选 deployment，再从部署历史中显式激活。重复上传不会修改已有 deployment。

支持 `zip`、`tar.gz` / `tgz`、`tar.xz` / `txz`、`tar.bz2` / `tbz2`、`tar` 与 `7z`。

### 2. Remote URL

在部署源卡片中选择 **Remote URL**，填写 HTTP(S) 地址并选择网络策略：

* **public**：默认策略，拒绝 loopback、私网、链路本地地址、DNS rebinding、自签 TLS，以及重定向到非公网目标。
* **trusted_internal**：仅用于明确受信的内网或自签服务；保存前需要再次确认风险。

保存后地址只以脱敏形式展示。编辑其它配置时无需重新填写；只有选择更换地址时才提交新 URL。Remote 来源只提供 **「同步并发布」**：每次由 Server 下载、校验并原子激活，不支持“检查更新”、定时检查或自动更新。

### 3. GitHub Release

GitHub 来源仅支持公开 `github.com` 仓库。填写：

* `https://github.com/{owner}/{repo}` 格式的仓库地址；
* **最新 Release** 或 **固定 Tag**；
* 精确、区分大小写的 Release Asset 文件名，默认 `dist.zip`。

两种选择都可手动 **「检查更新」** 和 **「同步并发布」**。区别如下：

* **latest**：可设置 5～1440 分钟检查间隔，默认 60 分钟；自动更新默认关闭。开启后，scanner 发现新 revision 才会异步同步并发布。
* **tag**：只支持管理员手动检查和同步，不参与定时 scanner。

“检查更新”只解析 Release/asset 并更新版本游标，不下载部署包；“同步并发布”才会下载、校验、创建或复用 deployment 并激活。如果同一个 Release 下的 asset 被替换，来源会进入 **「需要确认」**，必须确认页面显示的精确 revision 后才能发布，避免静默覆盖。

GitHub Release 在这里是预构建产物源，不等同于连接代码仓库自动构建。未来仓库集成会使用独立的 `git_repository` 来源和 Server build executor，再把构建产物送入同一部署管线。

### 4. 切换或删除来源

可以在手动、Remote 和 GitHub Release 之间切换。修改或删除来源不会删除当前生产部署和历史 deployment；切回手动模式后可继续上传并显式激活。

## 部署包安全限制

部署包必须满足以下约束：

* 压缩包大小由系统配置 `pages_max_package_size_mb` 控制，默认 100 MiB，可配置 1～2048 MiB。
* 展开后的单文件和总量上限为“包大小上限 × 4”，且最低为 100 MiB；最多 1,000 个常规文件。
* 控制面会流式读取常规文件体，核对声明大小与实际字节，并校验项目入口文件。
* 归档中的绝对路径、`..` 路径逃逸、软链接、硬链接和特殊文件都会被拒绝。

Agent 下载时还会执行 SHA-256、真实响应字节上限、解压后文件数与总大小复核；失败不会切换现有 `current`。

## 第三步：配置高级路由规则

### 1. SPA Fallback

使用 React Router、Vue Router 等前端路由时，开启 **「SPA Fallback」** 并设置入口路径（通常为 `/index.html`）。访客直接访问不存在的物理路径时，OpenResty 会回退到入口文件交由前端路由处理。

### 2. API 反向代理

Pages 可在同一域名下把指定前缀转发到后端 API：

* **APIProxyPath**：匹配前缀，例如 `/api`。
* **APIProxyPass**：后端地址，例如 `http://10.0.0.5:8080`。
* **APIProxyRewrite**：可选的路径重写规则。

匹配 API 前缀的请求走反向代理，其余请求继续由静态站点处理。

## 第四步：绑定路由并首次发布

1. 创建或编辑一条代理规则。
2. 将源站类型设为 **Pages**，并选择 Pages **项目**。
3. 预览配置后发布并激活。

路由绑定的是稳定的项目 ID，不是某个 deployment。首次发布让 Agent 获得项目锚点；此后本地上传、来源同步、自动更新或人工回滚只会改变项目的 active deployment，Agent 会通过 latest hash 对账收敛，无需重新发布主配置。

## 运维、状态与回滚

* 来源卡片展示最近检查/同步、已发现与已应用 revision、下次检查和安全错误。检查或同步任务运行时，页面会轮询任务状态；latest 空闲时只在接近检查时间时低频刷新。
* 自动更新失败不会替换旧 active deployment；单个来源失败也不会阻塞 scanner 处理其它项目。
* 在部署历史中激活其它 deployment 即完成人工回滚。系统会 fence 在途来源任务，并关闭该来源的自动更新，避免下一轮 latest 又覆盖人工选择；重复激活当前版本是 no-op。
* Agent 下载到临时文件并校验 SHA-256，安全解压后原子切换 `current`。任一步失败都保留旧内容，多项目对账时单项目失败不影响其它项目。

> [!TIP]
> 关于来源状态机、自动 scanner、上传补偿、不可变部署和 Agent 原子切换，请参阅 [Pages 静态托管设计](../design/pages-design.md)。
