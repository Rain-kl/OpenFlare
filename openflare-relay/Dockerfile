# syntax=docker/dockerfile:1.7
ARG VERSION=dev

FROM golang:1.25-alpine AS builder

ARG VERSION

WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY openflare-relay ./openflare-relay
COPY pkg ./pkg
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflare-relay/internal/config.Version=$VERSION'" -o /build/bin/openflare-relay ./openflare-relay/cmd/relay

# Final runtime image
FROM fatedier/frps:v0.69.0

WORKDIR /app

# Copy openflare-relay binary
COPY --from=builder /build/bin/openflare-relay ./openflare-relay

VOLUME ["/app/data"]

ENV OPENFLARE_FRPS_PATH=/usr/bin/frps
ENV OPENFLARE_DATA_DIR=/app/data

ENTRYPOINT ["/app/openflare-relay"]
