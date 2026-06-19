# syntax=docker/dockerfile:1.7
ARG VERSION=dev

FROM golang:1.25-alpine AS builder

ARG VERSION
ARG TARGETOS=linux
ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY openflare-agent ./openflare-agent
COPY pkg ./pkg
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflare-agent/internal/config.Version=$VERSION'" -o /build/bin/openflare-agent ./openflare-agent/cmd/agent

FROM openresty/openresty:alpine

RUN apk add --no-cache ca-certificates tzdata perl libmaxminddb \
    && ln -sf /usr/lib/libmaxminddb.so.0 /usr/lib/libmaxminddb.so \
    && opm get anjia0532/lua-resty-maxminddb \
    && mkdir -p /etc/openflare /data

ENV OPENFLARE_OPENRESTY_PATH=openresty \
    OPENFLARE_DATA_DIR=/data

COPY --from=builder /build/bin/openflare-agent /usr/local/bin/openflare-agent

EXPOSE 80 443 18081
ENTRYPOINT ["/usr/local/bin/openflare-agent"]
CMD ["-config", "/etc/openflare/agent.json"]
