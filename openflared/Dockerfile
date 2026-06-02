# syntax=docker/dockerfile:1.7
ARG VERSION=dev

FROM golang:1.25-alpine AS builder

ARG VERSION

WORKDIR /build

COPY openflared/go.mod openflared/go.sum ./
COPY openflare_server/go.mod openflare_server/go.sum ../openflare_server/
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY openflare_server/ ../openflare_server/
COPY openflared/ .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X 'openflare-flared/internal/config.Version=$VERSION'" -o flared ./cmd/flared

# Final runtime image
FROM fatedier/frpc:v0.69.0

WORKDIR /app

# Copy openflared binary
COPY --from=builder /build/flared .

ENV OPENFLARE_DATA_DIR=/app/data
ENV OPENFLARE_FRPC_PATH=/usr/bin/frpc

ENTRYPOINT ["/app/flared"]
CMD []

