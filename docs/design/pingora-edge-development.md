# Pingora 边缘节点开发说明

Pingora 节点在 `edge/pingora/` 中构建并连接现有主控。主控只保存一份业务快照，Pingora 和 OpenResty 分别编译或渲染这份快照。现有 OpenResty Agent 的协议在迁移期间继续可用。

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

Edge 通过活跃配置接口获取 `version`、`release_sequence`、`payload_checksum` 和 `payload_json`。载荷中的 `snapshot` 是业务快照，`assets` 携带已发布的证书及页面资源。主控使用 `source_version` 找到共享快照。节点校验载荷原始 UTF-8 字节的 SHA-256 摘要。当前 Edge 只接受统一快照契约，开发阶段的旧发布记录需要重新发布。

注册、心跳、应用结果和 Pages 下载沿用现有 Agent API。回滚和强制同步按节点引擎选择目标。变更通知触发主动拉取，应用与心跳读取数据面实际生效的版本。

## 构建和验证

```bash
cargo fmt --manifest-path edge/pingora/Cargo.toml --check
cargo clippy --manifest-path edge/pingora/Cargo.toml --locked --all-targets -- -D warnings
cargo test --manifest-path edge/pingora/Cargo.toml --locked
cargo build --manifest-path edge/pingora/Cargo.toml --locked
python3 edge/pingora/tests/runtime_smoke.py edge/pingora/target/debug/openflare-edge
```

烟测使用本地模拟主控和源站，覆盖注册、配置确认、HTTPS 跳转、按域名协商 HTTP/2、站点认证、PoW 浏览器协议、WAF IP 组、Pages 切换、缓存命中和离线重启。迁移验收还应在目标 Linux 环境对照 OpenResty 的客户端响应和源站请求，验证流式上传、SSE、WebSocket 与进程交接。

```bash
openflare-edge run \
  --server-url https://control.example.com \
  --token-type discovery \
  --token-file /etc/openflare-edge/bootstrap-token \
  --state-dir /var/lib/openflare-edge
```

预分配节点使用 `--token-type node`。已保存专属凭证的节点从状态目录恢复身份。Linux 安装包使用 `edge-v*` 标签，包含 `amd64` 和 `arm64` 产物。安装脚本保留上一二进制和有效快照，使用 systemd 就绪回执确认切换。
