# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /bin/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -g '' appuser \
  && apk add --no-cache ca-certificates tzdata curl

WORKDIR /app
COPY --from=builder /bin/server /app/server

USER appuser

EXPOSE 8080
ENV PORT=8080 APP_ENV=container

CMD ["/app/server"]
