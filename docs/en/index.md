---
layout: home

hero:
  name: OpenFlare
  text: Open-source CDN Orchestration & Edge Security Platform
  tagline: Supports reverse proxy, centralized configuration synchronization, secure intranet penetration (Tunnels), dynamic WAF protection, and anti-CC challenges.
  actions:
    - theme: brand
      text: Quick Start
      link: /en/guide/quick-start
    - theme: alt
      text: Design Boundaries
      link: /en/design/
    - theme: alt
      text: GitHub
      link: https://github.com/Rain-kl/OpenFlare

features:
  - icon: 🛰️
    title: Centralized Config Sync
    details: Sync configurations across all nodes in real time via WebSockets and heartbeats with sub-second hot reload. Instantly retrieve alerts and statuses.
  - icon: 🌐
    title: Distributed CDN Orchestration
    details: Orchestrate scattered and independent OpenResty nodes into a highly collaborative CDN fleet with website-level multi-domain aggregation and load balancing.
  - icon: 🚇
    title: Secure Intranet Penetration (Tunnels)
    details: An open-source alternative to Cloudflare Tunnels. Expose local intranet services securely to the public network without a public IP or open inbound ports.
  - icon: 🛡️
    title: Edge WAF Protection
    details: Dynamic WAF rules with differential syncing of IP groups to Lua shared memory without Nginx reloads, plus country-level regional access control.
  - icon: 🧩
    title: Anti-CC & Bot Defense (PoW)
    details: Built-in high-performance client-side cryptographic Proof of Work challenges (similar to Turnstile) to intercept botnets and scrapers at the edge.
---
