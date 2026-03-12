<p align="right">
    <a href="./README.md">中文</a> | <strong>English</strong>
</p>

<div align="center">

# ATSFlare

_✨ control plane for reverse proxy management ✨_

</div>

<p align="center">
  <a href="https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/LICENSE">
    <img src="https://img.shields.io/github/license/Rain-kl/ATSFlare?color=brightgreen" alt="license">
  </a>
  <a href="https://github.com/Rain-kl/ATSFlare/releases/latest">
    <img src="https://img.shields.io/github/v/release/Rain-kl/ATSFlare?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://github.com/Rain-kl/ATSFlare/releases/latest">
    <img src="https://img.shields.io/github/downloads/Rain-kl/ATSFlare/total?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://goreportcard.com/report/github.com/Rain-kl/ATSFlare">
    <img src="https://goreportcard.com/badge/github.com/Rain-kl/ATSFlare" alt="GoReportCard">
  </a>
</p>

## Repository layout

- `atsf_server`: Gin + GORM + SQLite control plane, including admin API, Agent API and Web UI
- `atsf_agent`: single-binary Go Agent for registration, heartbeat, config sync and OpenResty reload
- `docs`: design, development guidelines, implementation plan and deployment docs

## Quick start

### 1. Start the Server

The fastest way is to run the published GHCR image with Docker Compose:

```yaml
services:
  atsflare:
    image: ghcr.io/rain-kl/atsflare:latest
    container_name: atsflare
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-random-string
      SQLITE_PATH: /data/atsflare.db
      GIN_MODE: release
    volumes:
      - atsflare-data:/data

volumes:
  atsflare-data:
```

```bash
docker compose up -d
```

Default URL: `http://localhost:3000`

Notes:

* Replace `SESSION_SECRET` with a random string
* Replace `latest` with a fixed version tag if needed, for example `ghcr.io/rain-kl/atsflare:v0.3.0`
* SQLite data is persisted in the Docker volume `atsflare-data`
* For source build and full deployment steps, see [docs/deployment.md](./docs/deployment.md)

### 1.1 Server environment variables

Common environment variables:

| Variable | Description | Default behavior |
| --- | --- | --- |
| `PORT` | Server listen port | Defaults to `3000` |
| `GIN_MODE` | Gin runtime mode | Runs in release mode unless set to `debug` |
| `SESSION_SECRET` | Session signing secret; should be set explicitly in production | Falls back to the built-in default |
| `SQLITE_PATH` | SQLite database file path | Uses the built-in SQLite path |
| `SQL_DSN` | MySQL DSN; when set, MySQL is used instead of SQLite | Uses SQLite when unset |
| `REDIS_CONN_STRING` | Redis connection string for session/rate-limit related features | Redis is disabled when unset |
| `UPLOAD_PATH` | Upload directory path | Defaults to `upload` |
| `AGENT_TOKEN` | Legacy/global Agent token compatibility setting | Not required in the current default setup |

Example:

```bash
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./atsflare.db'
export GIN_MODE='release'
export PORT='3000'
```

### 2. Install Agent with Discovery Token

Use this for first-time node bootstrap. The Agent will auto-register with `discovery_token` and exchange it for a node-specific `agent_token`.

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

### 3. Install Agent with Agent Token

Use this when the node has already been created in the control plane and you already have a dedicated `agent_token`.

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

Notes:

* Replace `--server-url` with your actual control plane address, for example `http://192.168.1.10:3000`
* On Linux, the installer defaults to `/opt/atsflare-agent` and creates the `atsflare-agent` systemd service
* Re-running the same command upgrades the Agent to the latest release

## Delivery channels

Current release outputs:

* Server binaries: GitHub Releases
* Server Docker image: GitHub Container Registry at `ghcr.io/rain-kl/atsflare`
* Agent binaries: GitHub Releases

## Documentation

See:

1. [docs/deployment.md](./docs/deployment.md)
2. [docs/design.md](./docs/design.md)
3. [docs/development-guidelines.md](./docs/development-guidelines.md)
4. [docs/development-plan.md](./docs/development-plan.md)

Frontend notes:

* The new admin frontend lives in `atsf_server/web`
* `pnpm` is the default package manager
* `pnpm build` exports static assets into `atsf_server/web/build`