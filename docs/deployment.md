# GinNextTemplate 部署说明

本文档描述 `GinNextTemplate` 当前阶段的部署基线、启动方式和验证步骤。

当前模板工程仅描述服务端与前端的部署，不再包含 Agent、节点联调或 OpenResty 配置分发链路。

## 1. 前置条件

### 1.1 Server

* Go 1.24+
* Node.js 18+
* 可写 SQLite 文件目录，或可访问的 PostgreSQL 实例

## 2. 启动 Server

### 2.1 构建前端

```bash
cd openflare_server/web
corepack enable
pnpm install
pnpm build
```

`pnpm build` 会生成供 Go Server 托管的静态产物。

### 2.2 源码启动

```bash
cd openflare_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./ginnexttemplate.db'
export LOG_LEVEL='info'
# 可选：使用 PostgreSQL
# export DSN='postgres://template:secret@127.0.0.1:5432/ginnexttemplate?sslmode=disable'
go run .
```

默认监听 `3000` 端口。

### 2.3 Docker Compose 启动

```yaml
services:
  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ginnexttemplate
      POSTGRES_USER: ginnexttemplate
      POSTGRES_PASSWORD: replace-with-strong-password
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ginnexttemplate -d ginnexttemplate"]
      interval: 10s
      timeout: 5s
      retries: 5

  ginnexttemplate:
    image: your-registry/ginnexttemplate:latest
    container_name: ginnexttemplate
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-random-string
      SQLITE_PATH: /data/ginnexttemplate.db
      DSN: postgres://ginnexttemplate:replace-with-strong-password@postgres:5432/ginnexttemplate?sslmode=disable
      GIN_MODE: release
      LOG_LEVEL: info
    volumes:
      - ginnexttemplate-data:/data

volumes:
  postgres-data:
  ginnexttemplate-data:
```

```bash
docker compose up -d
```

## 3. 首次登录

访问 `http://localhost:3000`

默认账号：

* 用户名：`root`
* 密码：`123456`

## 4. Swagger

登录管理端后访问：

`http://localhost:3000/swagger/index.html`

如需重新生成文档：

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.4
cd openflare_server
swag init -g main.go -o docs
```

## 5. 最小验证步骤

1. 启动服务端
2. 登录管理端
3. 验证用户登录、文件上传、邮箱流程或系统设置页面可访问
4. 如启用升级功能，验证升级相关页面与接口可访问

## 6. 升级说明

当前模板工程保留服务端升级能力：

* 可检查最新版本
* 可上传服务端二进制进行手动升级
* 可执行服务端升级流程

当前模板工程不再描述 Agent 升级、节点升级或 OpenResty 重载。

## 7. 常用验证命令

### 7.1 Server

```bash
cd openflare_server
GOCACHE=/tmp/ginnexttemplate-go-cache go test ./...
```

### 7.2 Frontend

```bash
cd openflare_server/web
pnpm build
```

## 8. 文档维护要求

部署方式、升级方式、启动步骤或联调流程变化时，必须同步更新本文档和 `README.md`。
