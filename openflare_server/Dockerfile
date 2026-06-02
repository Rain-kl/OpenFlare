# syntax=docker/dockerfile:1.7
ARG VERSION=dev
FROM node:20 AS web-builder
ARG VERSION
WORKDIR /build
RUN corepack enable
COPY ./web/package.json ./web/pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
COPY ./web ./
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
COPY . .
COPY --from=web-builder /build/build ./web/build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags "-s -w -X 'openflare/common.Version=$VERSION'" \
      -o openflare

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates 2>/dev/null || true
ENV PORT=3000
COPY --from=go-builder /build/openflare /openflare
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/openflare"]
