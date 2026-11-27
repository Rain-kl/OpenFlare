# syntax=docker/dockerfile:1.7

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

RUN pnpm build:embed

FROM golang:alpine AS backend-builder

ARG VERSION=dev
ARG BUILD_DATE=""

ENV GOTOOLCHAIN=""

RUN apk add --no-cache ca-certificates git

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=bind,source=docker/go-toolchain.sh,target=/tmp/go-toolchain.sh \
    sh /tmp/go-toolchain.sh && go mod download

COPY . .
COPY --from=frontend-builder /workspace/frontend/out ./internal/router/root/dist

RUN CGO_ENABLED=0 GOOS=linux go build \
    -tags embed_frontend \
    -trimpath \
    -ldflags="-s -w -X github.com/Rain-kl/Wavelet/internal/platform/buildinfo.Version=${VERSION} -X github.com/Rain-kl/Wavelet/internal/platform/buildinfo.BuildTime=${BUILD_DATE}" \
    -o /out/openflare-server \
    ./cmd/server/main.go

FROM alpine:${ALPINE_VERSION} AS netintel-fetcher

ARG BUNDLE_NETINTEL=true
ARG NETINTEL_GEOIP_SOURCE=fyralabs

RUN apk add --no-cache bash ca-certificates coreutils curl gzip

WORKDIR /workspace
COPY scripts/fetch-netintel-dbs.sh ./scripts/fetch-netintel-dbs.sh

RUN mkdir -p /out/netintel && \
    if [ "${BUNDLE_NETINTEL}" = "true" ]; then \
      NETINTEL_OUT_DIR=/out/netintel NETINTEL_GEOIP_SOURCE="${NETINTEL_GEOIP_SOURCE}" bash ./scripts/fetch-netintel-dbs.sh; \
    fi

FROM alpine:${ALPINE_VERSION}

ARG TZ=Asia/Shanghai

RUN apk add --no-cache ca-certificates tzdata postgresql-client && \
    cp /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo "${TZ}" > /etc/timezone

WORKDIR /app

COPY --from=backend-builder /out/openflare-server ./openflare-server
COPY --from=netintel-fetcher /out/netintel ./data/netintel

EXPOSE 3000

ENTRYPOINT ["./openflare-server"]
CMD ["all"]
