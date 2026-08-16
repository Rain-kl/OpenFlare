# Credits

OpenFlare draws on the excellent ideas, architectures, and technical implementations of many open-source projects during design and development. Below are the key open-source projects referenced in OpenFlare's core underlying engines, security mechanisms, and frontend/backend frameworks. We thank these projects and their communities.

---

### 1. OpenResty
*   **Positioning**: a high-performance web platform based on Nginx and Lua.
*   **Role in OpenFlare**: the edge gateway of the global data plane. All public web traffic is first received by OpenResty, where high-concurrency HTTPS handshakes, WAF security rule matching, anti-CC human verification, and finally reverse proxy forwarding are performed.
*   **Link**: [OpenResty official site](https://openresty.org/)

### 2. FRP (Fast Reverse Proxy)
*   **Positioning**: a high-performance reverse proxy application focused on intranet penetration.
*   **Role in OpenFlare**: the underlying tunnel engine of the intranet penetration subsystem. The relay-side manager `openflare-relay` guards and schedules the `frps` engine, while the intranet client `openflared` auto-generates TOML config locally and guards multiplexed `frpc` child processes.
*   **Link**: [fatedier/frp (GitHub)](https://github.com/fatedier/frp)

---

### 3. Anubis (PoW solution)
*   **Positioning**: a lightweight human-verification protection solution based on Proof of Work.
*   **Role in OpenFlare**: provides the core **invisible anti-CC human challenge** capability for the gateway WAF.

---

### 4. gin-template
*   **Positioning**: a modern full-stack development scaffold template based on Go Gin and frontend builds.
*   **Role in OpenFlare**: provides a canonical, unified frontend/backend system architecture prototype for the OpenFlare control plane (Server).

---
