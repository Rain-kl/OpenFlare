# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
ARG NODE_VERSION=22
ARG ALPINE_VERSION=3.23

FROM node:${NODE_VERSION}-alpine AS frontend-builder

ARG VERSION=dev
ARG BUILD_DATE=""

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
ENV NODE_ENV=production
ENV NEXT_STANDALONE_EXPORT=true
ENV NEXT_PUBLIC_APP_VERSION=${VERSION}
ENV NEXT_PUBLIC_APP_BUILD_DATE=${BUILD_DATE}

RUN corepack enable && corepack prepare pnpm@10.10.0 --activate

WORKDIR /workspace/frontend

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./

RUN pnpm build:embed

FROM golang:${GO_VERSION}-alpine AS backend-builder

ARG VERSION=dev
ARG BUILD_DATE=""

RUN apk add --no-cache ca-certificates git

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /workspace/frontend/out ./internal/router/root/dist

RUN CGO_ENABLED=0 GOOS=linux go build \
    -tags embed_frontend \
    -trimpath \
    -ldflags="-s -w -X github.com/Rain-kl/Wavelet/internal/buildinfo.Version=${VERSION} -X github.com/Rain-kl/Wavelet/internal/buildinfo.BuildTime=${BUILD_DATE}" \
    -o /out/wavelet \
    ./main.go

FROM alpine:${ALPINE_VERSION}

ARG TZ=Asia/Shanghai

RUN apk add --no-cache ca-certificates tzdata postgresql-client && \
    cp /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo "${TZ}" > /etc/timezone

WORKDIR /app

COPY --from=backend-builder /out/wavelet ./wavelet
COPY docs ./docs

EXPOSE 3000

ENTRYPOINT ["./wavelet"]
CMD ["all"]
