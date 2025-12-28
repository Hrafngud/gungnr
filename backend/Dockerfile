# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /bin/server ./cmd/server

FROM alpine:3.20
ARG TARGETARCH

RUN adduser -D -g '' appuser \
  && apk add --no-cache ca-certificates tzdata curl git docker-cli docker-cli-compose \
  && ARCH="${TARGETARCH:-}" \
  && if [ -z "$ARCH" ]; then ARCH="$(uname -m)"; fi \
  && case "$ARCH" in \
      amd64|x86_64) CLOUDFLARED_ARCH=amd64 ;; \
      arm64|aarch64) CLOUDFLARED_ARCH=arm64 ;; \
      *) echo "unsupported arch: $ARCH" && exit 1 ;; \
    esac \
  && curl -L --output /usr/local/bin/cloudflared "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-$CLOUDFLARED_ARCH" \
  && chmod +x /usr/local/bin/cloudflared

WORKDIR /app
COPY --from=builder /bin/server /app/server

USER appuser

EXPOSE 8080
ENV PORT=8080 APP_ENV=container

CMD ["/app/server"]
