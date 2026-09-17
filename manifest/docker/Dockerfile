# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
ARG NODE_VERSION=22
ARG ALPINE_VERSION=3.23

FROM node:${NODE_VERSION}-alpine AS frontend-builder

ARG VERSION=dev
ARG BUILD_DATE=""

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
ENV NODE_ENV=production
ENV NEXT_STANDALONE_EXPORT=true
ENV NEXT_PUBLIC_APP_VERSION=${VERSION}
ENV NEXT_PUBLIC_APP_BUILD_DATE=${BUILD_DATE}

RUN corepack enable && corepack prepare pnpm@10.10.0 --activate

WORKDIR /workspace/frontend

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./

# Assert index.html so a silent export failure cannot ship an API-only binary.
RUN pnpm build:embed && test -f out/index.html

FROM golang:${GO_VERSION}-alpine AS backend-builder

ARG VERSION=dev
ARG BUILD_DATE=""

RUN apk add --no-cache ca-certificates git

WORKDIR /workspace

# Cache dependency layer
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY . .

# Drop any stale bundle carried in the context, then overlay the fresh export so
# `//go:embed all:dist` in driver_http picks up exactly this frontend build.
RUN rm -rf backend/plugins/drivers/driver_http/dist
COPY --from=frontend-builder /workspace/frontend/out backend/plugins/drivers/driver_http/dist

WORKDIR /workspace/backend

RUN test -f plugins/drivers/driver_http/dist/index.html && \
    CGO_ENABLED=0 GOOS=linux go build \
    -tags embed_frontend \
    -trimpath \
    -ldflags="-s -w -X Wavelet/pkg/buildinfo.Version=${VERSION} -X Wavelet/pkg/buildinfo.BuildTime=${BUILD_DATE}" \
    -o /out/openflare-server \
    ./main.go

FROM alpine:${ALPINE_VERSION}

ARG TZ=Asia/Shanghai

RUN apk add --no-cache ca-certificates tzdata postgresql-client && \
    cp /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo "${TZ}" > /etc/timezone

WORKDIR /app

COPY --from=backend-builder /out/openflare-server ./openflare-server
COPY docs ./docs

EXPOSE 3000

ENTRYPOINT ["./openflare-server"]
CMD ["all"]
