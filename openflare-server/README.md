# wavelet

🚀 A modern, production-ready full-stack boilerplate for building scalable web applications

[中文](./README_zh.md)

[![License: Apache2.0](https://img.shields.io/badge/License-Apache2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-16-black.svg)](https://nextjs.org/)
[![React](https://img.shields.io/badge/React-19-blue.svg)](https://reactjs.org/)

## 📖 Introduction

**wavelet** is a generic, production-ready full-stack boilerplate built with **Go (Gin + GORM)** on the backend and **Next.js (App Router + Shadcn UI)** on the frontend. It ships with everything you need to bootstrap a modern SaaS, internal tool, or developer platform — without the boilerplate headaches.

The project was designed from the ground up to be **framework-first and business-agnostic**: plug in your own domain logic while reusing the battle-tested infrastructure that comes out of the box.

### ✨ Key Features

- 🔐 **Multi-auth System** — Local password login/registration + pluggable OIDC/OAuth2 providers (supports multiple auth sources simultaneously)
- 🗝️ **Personal Access Tokens** — API key management for programmatic access; supports `Authorization: Bearer` and `X-Access-Token` headers
- 👤 **User Management** — Admin panel for listing, searching, filtering, enabling/disabling user accounts
- ⚙️ **Dynamic System Config** — Key-value system configuration management with live reload, controllable from the admin UI
- 📋 **Async Task Queue** — Background job processing with [Asynq](https://github.com/hibiken/asynq) (Redis-backed), including a scheduling dashboard
- 📁 **S3 File Storage** — Unified file upload/download via S3-compatible APIs with local disk cache
- 📊 **Observability** — Structured logging (Zap) + distributed tracing (OpenTelemetry)
- 🎨 **Modern UI** — Responsive, dark-mode-ready design system built with Tailwind CSS 4 and Shadcn UI
- 📖 **Built-in Documentation** — Integrated docs portal with usage guides, API reference, privacy policy, and terms of service

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌─────────────────────────────┐    ┌─────────────────┐
│   Frontend      │    │          Backend             │    │   Database      │
│   (Next.js)     │◄──►│           (Go)               │◄──►│  (PostgreSQL)   │
│                 │    │                              │    │                 │
│ • React 19      │    │ • Gin HTTP Framework         │    │ • PostgreSQL    │
│ • TypeScript    │    │ • GORM ORM                   │    │ • Redis Cache   │
│ • Tailwind 4    │    │ • Multi-provider Auth        │    │                 │
│ • Shadcn UI     │    │ • AccessToken Middleware     │    │                 │
│                 │    │ • Asynq Task Queue           │    │                 │
│                 │    │ • OpenTelemetry Tracing      │    │                 │
│                 │    │ • Swagger API Docs           │    │                 │
└─────────────────┘    └─────────────────────────────┘    └─────────────────┘
                                      │
                           ┌──────────┴──────────┐
                           │   Multi-Process CLI  │
                           │  (Cobra + Viper)     │
                           │ • api      (HTTP)    │
                           │ • worker   (Queue)   │
                           │ • scheduler(Cron)    │
                           └─────────────────────┘
```

## 🛠️ Tech Stack

### Backend
- **[Go 1.25+](https://go.dev/doc)** — Primary language
- **[Gin](https://github.com/gin-gonic/gin)** — HTTP web framework
- **[GORM](https://github.com/go-gorm/gorm)** — ORM with PostgreSQL & ClickHouse support
- **[Redis](https://github.com/redis/redis)** — Cache, session store, and task queue backend
- **[Asynq](https://github.com/hibiken/asynq)** — Distributed task queue (Redis-backed)
- **[Cobra + Viper](https://github.com/spf13/cobra)** — CLI entrypoint and configuration management
- **[OpenTelemetry](https://opentelemetry.io)** — Distributed tracing and observability
- **[Zap](https://github.com/uber-go/zap)** — Structured, high-performance logging
- **[Swagger (Swaggo)](https://github.com/swaggo/swag)** — Auto-generated API documentation
- **[AWS SDK v2](https://github.com/aws/aws-sdk-go-v2)** — S3-compatible file storage
- **[Snowflake](https://github.com/bwmarrin/snowflake)** — Distributed ID generation

### Frontend
- **[Next.js 16](https://github.com/vercel/next.js)** — React framework with App Router
- **[React 19](https://github.com/facebook/react)** — UI library
- **[TypeScript](https://github.com/microsoft/TypeScript)** — Type safety
- **[Tailwind CSS 4](https://github.com/tailwindlabs/tailwindcss)** — Utility-first styling
- **[Shadcn UI](https://github.com/shadcn-ui/ui)** — Accessible, composable component library
- **[Lucide Icons](https://github.com/lucide-icons/lucide)** — Icon library

## 📋 Requirements

- **Go** >= 1.25
- **Node.js** >= 18.0
- **PostgreSQL** >= 14
- **Redis** >= 6.0
- **pnpm** >= 8.0 (recommended)

## 🚀 Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/Rain-kl/Wavelet.git refreshing
cd refreshing
```

### 2. Configure Environment

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` to configure your database and Redis. OIDC auth sources are configured at runtime in the admin settings page.

### 3. Initialize Database

```bash
# Start local dependencies (PostgreSQL + Redis)
docker compose up -d

# Optional: also start ClickHouse
docker compose --profile clickhouse up -d

# If you use an external PostgreSQL instance instead of Docker, create the database manually
createdb -h <host> -p 5432 -U postgres refreshing

# Database schema is auto-migrated on first startup
```

### 4. Start the Backend

```bash
# Install Go dependencies
go mod tidy

# Generate Swagger API documentation
make swagger

# Start the HTTP API server
go run main.go api
```

> The backend also supports separate `scheduler` and `worker` processes for async task processing:
> ```bash
> go run main.go scheduler   # Cron job scheduler
> go run main.go worker      # Asynq task worker
> ```

### 5. Start the Frontend

```bash
cd frontend

# Install dependencies
pnpm install

# Start dev server (Turbopack)
pnpm dev
```

### 6. Access the Application

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| Swagger API Docs | http://localhost:8000/swagger/index.html |
| Health Check | http://localhost:8000/api/health |

## ⚙️ Configuration

Key configuration options (see `config.example.yaml` for the full reference):

| Option | Description | Example |
|--------|-------------|---------|
| `app.addr` | Backend listen address | `:8000` |
| `database.host` | PostgreSQL host | `127.0.0.1` |
| `database.database` | Database name | `refreshing` |
| `redis.host` | Redis host | `127.0.0.1` |
| `storage.endpoint` | S3-compatible endpoint | `s3.amazonaws.com` |

## 🔧 Development Guide

### Backend

```bash
# Run API server
go run main.go api

# Run task scheduler
go run main.go scheduler

# Run async worker
go run main.go worker

# Regenerate Swagger docs (required after controller changes)
make swagger

# Format & vet code
make tidy
```

### Frontend

```bash
cd frontend

# Development mode (Turbopack)
pnpm dev

# Production build
pnpm build

# Start production server
pnpm start

# Lint & format
pnpm lint
pnpm format
```

## 📁 Project Structure

```
wavelet/
├── main.go                  # Entry point (delegates to internal/cmd)
├── config.example.yaml      # Configuration template
├── Makefile                 # Common commands (swagger, tidy, license, cross-build)
├── docker/                  # Docker image build files (integrated/frontend/backend)
├── docs/                    # Swagger auto-generated docs
├── frontend/                # Next.js frontend application
│   ├── app/                 # App Router pages
│   ├── components/          # React components (ui, common, layout)
│   ├── lib/services/        # API service layer
│   └── types/               # TypeScript type definitions
└── internal/                # Go backend (private)
    ├── cmd/                 # CLI commands (api, scheduler, worker)
    ├── apps/                # Business modules (oauth, user, admin, upload)
    ├── model/               # GORM entities and business methods
    ├── router/              # HTTP route registration
    ├── task/                # Async task definitions and workers
    ├── db/                  # Database and Redis initialization
    ├── storage/             # S3 file storage abstraction
    └── common/              # Shared utilities and response helpers
```

## 📚 API Documentation

Swagger API documentation is auto-generated and available once the backend is running:

```
http://localhost:8000/swagger/index.html
```

The built-in frontend docs portal at `/docs` includes:
- **Usage Guide** — Step-by-step walkthrough for getting started
- **API Reference** — Detailed interface documentation
- **Privacy Policy** — Template privacy policy (customize as needed)
- **Terms of Service** — Template terms of service

## 🧪 Testing

```bash
# Backend tests
go test ./...

# Frontend lint
cd frontend && pnpm lint
```

## 🚀 Deployment

### Cross-platform Binary

Build static binaries for all 6 targets (Linux / macOS / Windows × amd64 / arm64) with a single command.
The compiled frontend is embedded in every binary — no separate deployment needed.

**Prerequisites:** Docker with BuildKit enabled (Docker 23+ defaults to on).

```bash
# Build all 6 binaries → ./bin/
make cross-build

# Stamp a release version
make cross-build VERSION=v1.2.3

# Build only a specific OS (both architectures)
make cross-build GOOS=linux
make cross-build GOOS=darwin
make cross-build GOOS=windows

# Build only a specific architecture (all OSes)
make cross-build GOARCH=amd64
make cross-build GOARCH=arm64

# Combine filters — single binary
make cross-build GOOS=linux GOARCH=arm64
make cross-build GOOS=darwin GOARCH=amd64 VERSION=v1.2.3
```

Output files in `./bin/`:

| File | Platform |
|------|----------|
| `wavelet_linux_amd64` | Linux x86-64 |
| `wavelet_linux_arm64` | Linux ARM64 |
| `wavelet_darwin_amd64` | macOS Intel |
| `wavelet_darwin_arm64` | macOS Apple Silicon |
| `wavelet_windows_amd64.exe` | Windows x86-64 |
| `wavelet_windows_arm64.exe` | Windows ARM64 |

> The version string is accessible at runtime via `wavelet --version`.

### Docker

```bash
# Build image
docker build -t refreshing .

# Run (pass your config as a volume mount)
docker run -d -p 8000:8000 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  refreshing api
```

### Production

1. Build the frontend:
   ```bash
   cd frontend && pnpm build
   ```

2. Compile the backend:
   ```bash
   go build -o refreshing main.go
   ```

3. Configure `config.yaml` for production.

4. Start services:
   ```bash
   ./refreshing api        # HTTP API
   ./refreshing scheduler  # Cron scheduler (optional)
   ./refreshing worker     # Task worker (optional)
   ```

## 🤝 Contributing

We welcome contributions! Please read the following before submitting code:

- [Contributing Guidelines](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Contributor License Agreement](CLA.md)

### Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -am 'Add your feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the [Apache 2.0 License](LICENSE).
