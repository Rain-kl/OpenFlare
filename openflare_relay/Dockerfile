FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY openflare_relay/go.mod openflare_relay/go.sum ./
COPY openflare_server /openflare_server
COPY openflare_relay /openflare_relay

WORKDIR /openflare_relay
RUN CGO_ENABLED=0 GOOS=linux go build -o openflare-relay ./cmd/relay

# Extract frps binary from official image
FROM fatedier/frps:v0.69.0 AS frps

# Final runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

# Copy openflare-relay binary
COPY --from=builder /openflare_relay/openflare-relay /usr/local/bin/openflare-relay

# Copy frps binary from official image
COPY --from=frps /usr/bin/frps /usr/local/bin/frps

VOLUME ["/var/lib/openflare-relay"]

ENV OPENFLARE_FRPS_PATH=/usr/local/bin/frps
ENV OPENFLARE_DATA_DIR=/var/lib/openflare-relay

ENTRYPOINT ["/usr/local/bin/openflare-relay"]
CMD ["-config", "/etc/openflare-relay/relay.json"]
