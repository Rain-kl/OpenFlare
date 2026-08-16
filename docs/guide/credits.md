# 引用与致谢

OpenFlare 在设计与实现过程中借鉴了众多开源项目的优秀理念、架构设计和技术实现。以下是 OpenFlare 在核心底层引擎、安全防护机制以及前后端系统框架等方面所引用的关键开源项目，在此对这些项目及其社区表示感谢。

---

### 1. OpenResty
*   **项目定位**：基于 Nginx 与 Lua 的高性能 Web 平台。
*   **在 OpenFlare 中的作用**：作为全局数据面（Data Plane）的边缘网关。所有的公网 Web 流量均首先由 OpenResty 接收，在此处进行高并发的 HTTPS 握手、WAF 安全规则比对、防 CC 人机验证，并最终执行反向代理转发。
*   **项目链接**：[OpenResty 官网](https://openresty.org/)

### 2. FRP（Fast Reverse Proxy）
*   **项目定位**：高性能的反向代理应用，专注于内网穿透。
*   **在 OpenFlare 中的作用**：作为内网穿透子系统的底层隧道引擎。中继端管理器 `openflare-relay` 负责守护和调度 `frps` 引擎，而内网客户端 `openflared` 则负责在本地自动生成 TOML 配置并守护多路复用 `frpc` 子进程。
*   **项目链接**：[fatedier/frp (GitHub)](https://github.com/fatedier/frp)

---

### 3. Anubis（PoW 方案）
*   **项目定位**：基于工作量证明（Proof of Work）的轻量级人机验证防护方案。
*   **在 OpenFlare 中的作用**：为网关 WAF 提供了核心的**无感防 CC 人机挑战**能力。

---

### 4. gin-template
*   **项目定位**：基于 Go Gin 与前端构建的现代化全栈开发脚手架模板。
*   **在 OpenFlare 中的作用**：为 OpenFlare 控制面（Server）提供了规范、统一的前后端系统架构雏形。

---
