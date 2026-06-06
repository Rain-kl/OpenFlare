# syntax=docker/dockerfile:1.7
ARG VERSION=dev
FROM node:20 AS web-builder
ARG VERSION
WORKDIR /build
RUN corepack enable
COPY openflare-server/web/package.json openflare-server/web/pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
COPY openflare-server/web ./
RUN --mount=type=cache,id=next-cache,target=/build/.next/cache \
    NEXT_PUBLIC_APP_VERSION="$VERSION" pnpm build

FROM golang:1.25 AS go-builder
ARG VERSION
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY openflare-server ./openflare-server
COPY --from=web-builder /build/build ./openflare-server/web/build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflare-server/common.Version=$VERSION'" \
      -o openflare ./openflare-server

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates 2>/dev/null || true
ENV PORT=3000
COPY --from=go-builder /build/openflare /openflare
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/openflare"]
