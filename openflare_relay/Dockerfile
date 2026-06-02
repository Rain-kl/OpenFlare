# syntax=docker/dockerfile:1.7
ARG VERSION=dev

FROM golang:1.25-alpine AS builder

ARG VERSION

WORKDIR /build
COPY openflare_server/go.mod openflare_server/go.sum /openflare_server/
COPY openflare_relay/go.mod openflare_relay/go.sum /openflare_relay/
WORKDIR /openflare_relay
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /build
COPY openflare_server /openflare_server
COPY openflare_relay /openflare_relay
WORKDIR /openflare_relay
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X 'openflare-relay/internal/config.Version=$VERSION'" -o openflare-relay ./cmd/relay

# Final runtime image
FROM fatedier/frps:v0.69.0

# Copy openflare-relay binary
COPY --from=builder /openflare_relay/openflare-relay /usr/local/bin/openflare-relay

VOLUME ["/var/lib/openflare-relay"]

ENV OPENFLARE_FRPS_PATH=/usr/bin/frps
ENV OPENFLARE_DATA_DIR=/var/lib/openflare-relay

ENTRYPOINT ["/usr/local/bin/openflare-relay"]

