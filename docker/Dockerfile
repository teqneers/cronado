FROM golang:1.25-alpine AS go-builder

WORKDIR /opt

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cronado main.go

FROM alpine:latest

ARG DOCKER_GID=999

WORKDIR /opt

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S app \
    && adduser -S -G app app \
    && addgroup app ping

USER app

EXPOSE 8080

ENV GIN_MODE=release

COPY --from=go-builder /opt/cronado ./

LABEL org.opencontainers.image.source="https://github.com/teqneers/cronado"
LABEL org.opencontainers.image.description="Lightweight cron job scheduler for Docker containers"
LABEL org.opencontainers.image.licenses="MIT"

CMD ["./cronado", "cron"]