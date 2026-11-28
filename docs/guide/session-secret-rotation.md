# 轮换 Session Secret

OpenFlare Server 使用 `app.session_secret` 签名会话并加密 TLS 私钥、DNS 凭证、ACME 密钥与 Cloudflare 令牌（MCP 要求至少 32 字节）；更换密钥前必须先用 `rotate-credentials` 重加密库内记录。

## 1. 生成新密钥并预检

在原配置或 `APP_SESSION_SECRET` 环境变量保持生效的条件下执行预检（环境变量优先于配置文件）：

```sh
rotation_secret_file="$HOME/.openflare-new-session-secret"
(umask 077; set -C; openssl rand -hex 32 > "$rotation_secret_file")
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file"
```

预检输出待迁移记录数；若全部待迁移计数为 `0` 可直接跳至第 3 步；若提示解密失败，先核对旧密钥与目标数据库是否匹配。

## 2. 停服、备份并执行重加密

1. 停止连接该主库的所有 OpenFlare Server 进程（`api`、`worker`、`scheduler`），保持数据库与 Redis 运行。
2. 备份旧密钥配置与主数据库（SQLite 示例：`(umask 077; sqlite3 /actual/path/openflare.db '.backup openflare-before-rotation.db')`；PostgreSQL 使用 `pg_dump`）。
3. 在单事务内完成重加密与解密校验：

```sh
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file" --apply
```

Docker Compose 环境（服务名 `openflare`）先执行 `docker compose stop openflare`，再通过一次性容器执行（去掉 `--apply` 即为预检）：

```sh
docker compose run --rm --no-deps \
  -v "$rotation_secret_file:/run/secrets/new-session-secret:ro" \
  openflare rotate-credentials --new-secret-file /run/secrets/new-session-secret --apply
```

## 3. 更新配置、重启与回滚

1. 在所有 Server 实例同步更新 `app.session_secret` 或 `APP_SESSION_SECRET` 后重启服务（Docker Compose 执行 `docker compose up -d openflare`）。
2. 登录控制台检查 **MCP** 标签页状态，并为既有 OAuth CLI / MCP 连接重新授权，确认无误后删除 `$rotation_secret_file`。
3. 如需回滚，须先停服并同时恢复迁移前数据库备份与旧密钥；仅还原密钥会导致已重加密凭证无法解密。
