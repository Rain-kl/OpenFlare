ARG VERSION=dev

FROM golang:1.25-alpine AS builder

ARG VERSION
ARG TARGETOS=linux
ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

WORKDIR /build
COPY openflare_server ./openflare_server
COPY openflare_agent ./openflare_agent
WORKDIR /build/openflare_agent
RUN go mod download
RUN go build -trimpath -ldflags "-s -w -X 'openflare-agent/internal/config.Version=$VERSION'" -o /build/openflare-agent ./cmd/agent

FROM openresty/openresty:alpine

RUN apk add --no-cache ca-certificates tzdata perl libmaxminddb \
    && ln -sf /usr/lib/libmaxminddb.so.0 /usr/lib/libmaxminddb.so \
    && opm get anjia0532/lua-resty-maxminddb \
    && mkdir -p /etc/openflare /data

ENV OPENFLARE_OPENRESTY_PATH=openresty \
    OPENFLARE_DATA_DIR=/data

COPY --from=builder /build/openflare-agent /usr/local/bin/openflare-agent

EXPOSE 80 443 18081
ENTRYPOINT ["/usr/local/bin/openflare-agent"]
CMD ["-config", "/etc/openflare/agent.json"]
