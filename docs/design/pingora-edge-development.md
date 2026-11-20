# Pingora 边缘节点开发说明

Pingora 节点在 `edge/pingora/` 中构建并连接主控。

## 包结构

| 模块 | 职责 |
| --- | --- |
| `edge/pingora/src/main.rs`、`runtime/` | 启动参数、组件装配和监听就绪 |
| `protocol/` | 解码统一快照契约并转换运行参数 |
| `control/` | 注册、身份、心跳、WebSocket 通知、配置和资源同步 |
| `config.rs` | 候选校验、快照恢复、持久化和内存发布 |
| `proxy/` | Pingora 监听、TLS、请求阶段、回源、缓存与响应处理 |
| `policy/` | 认证、PoW、WAF、限流和 Service Worker |
| `pages/`、`geoip.rs`、`telemetry/` | 内容部署与静态服务、GeoIP 数据和节点上报 |
| `internal/openflare/edgeconfig/` | 在主控使用公共快照和已发布资源生成 Edge 传输载荷 |

## 配置接口

心跳返回版本化资源清单。WebSocket 的 `config_changed` 事件触发即时心跳。小配置直接出现在清单中，大配置按 SHA-256 从固定接口拉取。资源提供者和同步约定见 [Pingora 边缘节点设计](./pingora-edge-design.md#配置同步)。

## 构建和验证

```bash
cargo fmt --manifest-path edge/pingora/Cargo.toml --check
cargo clippy --manifest-path edge/pingora/Cargo.toml --locked --all-targets -- -D warnings
cargo test --manifest-path edge/pingora/Cargo.toml --locked
cargo build --manifest-path edge/pingora/Cargo.toml --locked
python3 edge/pingora/tests/runtime_smoke.py edge/pingora/target/debug/openflare-edge
```

烟测使用本地模拟主控和源站，覆盖注册、配置确认、HTTPS 跳转、按域名协商 HTTP/2、站点认证、PoW 浏览器协议、WAF IP 组、Pages 切换、缓存命中和离线重启。烟测也覆盖 WebSocket 唤醒、按哈希拉取、性能参数热更新和工作线程自动重载。

```bash
openflare-edge run \
  --server-url https://control.example.com \
  --token-type discovery \
  --token-file /etc/openflare-edge/bootstrap-token \
  --state-dir /var/lib/openflare-edge
```

预分配节点使用 `--token-type node`。已保存专属凭证的节点从状态目录恢复身份。Linux 安装包使用 `edge-v*` 标签，包含 `amd64` 和 `arm64` 产物。安装和更新脚本原子替换单个二进制，并使用 systemd 就绪回执验证启动。节点从独立的状态目录恢复有效快照。
