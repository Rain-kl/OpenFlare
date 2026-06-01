FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY openflared/go.mod openflared/go.sum ./
COPY openflare_server/go.mod openflare_server/go.sum ../openflare_server/
RUN go mod download

COPY openflare_server/ ../openflare_server/
COPY openflared/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o flared ./cmd/flared

# Extract frpc binary from official image
FROM fatedier/frpc:v0.69.0 AS frpc

# Final runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy openflared binary
COPY --from=builder /build/flared .

# Copy frpc binary from official image
COPY --from=frpc /usr/bin/frpc /usr/local/bin/frpc

ENV OPENFLARE_DATA_DIR=/app/data
ENV OPENFLARE_FRPC_PATH=/usr/local/bin/frpc

ENTRYPOINT ["/app/flared"]
CMD []
