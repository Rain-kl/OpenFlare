---
title: 更换会话密钥
description: 使用 OpenFlare Server 命令检查并重新加密存量凭据，然后切换会话密钥。
---

# 更换会话密钥

OpenFlare Server 使用 `app.session_secret` 签名浏览器会话，并加密 TLS 证书私钥、DNS 凭据、ACME 账户私钥和 Cloudflare 独立令牌。MCP 要求密钥至少 32 字节。`rotate-credentials` 会检查数据库中受旧密钥保护的凭据，并可在一个事务内用新密钥重新加密。

## 1. 生成新密钥并预检

使用与当前 Server 相同的配置、环境变量、工作目录和主数据库。此时保留原来的 `app.session_secret` 或 `APP_SESSION_SECRET`。如果两者同时设置，环境变量优先。

```sh
rotation_secret_file="$HOME/.openflare-new-session-secret"
(umask 077; set -C; openssl rand -hex 32 > "$rotation_secret_file")
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file"
```

预检按凭据字段显示待迁移数和已使用新密钥的记录数，数据库中的凭据保持原样。待迁移数全部为零时，可跳到第 3 步。遇到解密错误时，输出会标出记录 ID。先核对旧密钥和数据库，确认预检通过后再继续。

## 2. 停服务、备份并迁移

停止连接该数据库的所有 OpenFlare Server 实例，包括 API、Worker 和调度器。保留数据库与 Redis 等依赖服务，备份主数据库及包含旧密钥的配置。SQLite 可用 `(umask 077; sqlite3 /实际路径/openflare.db '.backup openflare-before-rotation.db')`，PostgreSQL 使用对应实例的 `pg_dump`。

```sh
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file" --apply
```

命令确认所有凭据都可以用新密钥解密后才提交。运行失败时，修正错误后可使用同一密钥文件重试。若需确认上次是否写入成功，可再次预检，检查待迁移数是否为零。

### Docker Compose

在使用 `openflare` 作为服务名的部署中，先运行 `docker compose stop openflare`，然后用一次性容器执行命令，复用原服务的数据库配置和卷：

```sh
docker compose run --rm --no-deps \
  -v "$rotation_secret_file:/run/secrets/new-session-secret:ro" \
  openflare rotate-credentials --new-secret-file /run/secrets/new-session-secret --apply
```

确认容器的运行用户可读取密钥文件。去掉 `--apply` 即为预检。拆分部署时，同时停止其他 OpenFlare Server 进程。

## 3. 切换配置并启动

1. 将密钥文件中的值写入有效的 `app.session_secret` 或 `APP_SESSION_SECRET`，并给所有 Server 实例使用同一个值。
2. 启动 Server。Docker Compose 可使用 `docker compose up -d openflare` 使环境变量生效。
3. 重新登录控制台，确认 MCP 设置可以读取。若已使用 OAuth 连接 CLI 或 MCP，重新授权。
4. 保留受保护的备份和旧密钥，清理迁移时使用的临时密钥文件。

若需要恢复，停止 Server，将**迁移前的数据库备份和旧密钥**一并还原，再启动服务。只切回旧密钥会导致已迁移的凭据无法解密。
