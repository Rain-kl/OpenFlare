ARG VERSION=dev

FROM node:18 AS builder

ARG VERSION

WORKDIR /build
COPY ./web/package*.json ./
RUN npm install
COPY ./web ./
RUN REACT_APP_VERSION="$VERSION" npm run build

FROM golang:1.22 AS builder2

ARG VERSION

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build
COPY . .
COPY --from=builder /build/build ./web/build
RUN go mod download
RUN go build -trimpath -ldflags "-s -w -X 'atsflare/common.Version=$VERSION'" -o atsflare

FROM alpine

RUN apk update \
    && apk upgrade \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates 2>/dev/null || true
ENV PORT=3000
COPY --from=builder2 /build/atsflare /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/atsflare"]
