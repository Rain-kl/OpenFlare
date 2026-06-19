# syntax=docker/dockerfile:1.7
ARG VERSION=dev

FROM golang:1.25-alpine AS builder

ARG VERSION

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY openflared/ ./openflared/
COPY pkg/ ./pkg/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflared/internal/config.Version=$VERSION'" -o flared ./openflared/cmd/flared

# Final runtime image
FROM fatedier/frpc:v0.69.0

WORKDIR /app

# Copy openflared binary
COPY --from=builder /build/flared .

ENV OPENFLARE_DATA_DIR=/app/data
ENV OPENFLARE_FRPC_PATH=/usr/bin/frpc

ENTRYPOINT ["/app/flared"]
CMD []
