# 引用与致谢

OpenFlare 在设计与实现过程中借鉴了众多开源项目的优秀理念、架构设计和技术实现。以下是 OpenFlare 在核心底层引擎、安全防护机制以及前后端系统框架等方面所引用的关键开源项目，在此对这些项目及其社区表示感谢。

---

### 1. Pingora 与 Pingap
*   **项目定位**：Cloudflare 开源的 Rust 异步多线程高性能网络代理框架及 Pingap 缓存实现。
*   **在 OpenFlare 中的作用**：作为全局数据面（Data Plane）的边缘网关核心引擎。边缘节点（`openflare-edge`）基于 Pingora 构建，负责接收全部公网 Web 流量，执行动态 TLS 握手与证书管理、WAF 安全规则比对、PoW 客户端人机验证、两级边缘缓存与反向代理转发。
*   **项目链接**：[cloudflare/pingora (GitHub)](https://github.com/cloudflare/pingora) 与 [vicanso/pingap (GitHub)](https://github.com/vicanso/pingap)

---

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
