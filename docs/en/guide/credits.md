# Credits

OpenFlare is essentially a solution integration project. During its design and implementation phases, it drew inspiration from the exceptional concepts, architectural designs, and technical achievements of numerous open-source projects. Below are the key upstream open-source projects OpenFlare relies on for its core engine, security mechanisms, and backend/frontend system frameworks, along with our sincere thanks to these projects and their active communities.

---

### 1. OpenResty
* **Project Positioning**: A high-performance Web platform based on Nginx and Lua.
* **Role in OpenFlare**: Acts as the edge gateway for the global Data Plane. All public web traffic is received by OpenResty first, where high-concurrency HTTPS handshakes, WAF security rule evaluations, and PoW CC verification are performed before executing reverse proxies.
* **Project Link**: [OpenResty Official Website](https://openresty.org/)

### 2. FRP (Fast Reverse Proxy)
* **Project Positioning**: A high-performance reverse proxy application focused on intranet penetration.
* **Role in OpenFlare**: Serves as the underlying tunnel engine for the intranet penetration subsystem. The relay-side manager `openflare-relay` is responsible for running and scheduling the `frps` engine, while the intranet client `openflared` is responsible for generating TOML configurations locally and running the multiplexed `frpc` subprocesses.
* **Project Link**: [fatedier/frp (GitHub)](https://github.com/fatedier/frp)

---

### 3. Anubis (PoW Solution)
* **Project Positioning**: A lightweight human-machine verification and protection solution based on Proof of Work (PoW).
* **Role in OpenFlare**: Provides the core **seamless PoW CC challenge** capabilities for the gateway WAF.

---

### 4. gin-template
* **Project Positioning**: A modern full-stack development boilerplate based on Go Gin and frontend builds.
* **Role in OpenFlare**: Provided the standard, unified backend/frontend system architecture baseline for the OpenFlare control plane (Server).
