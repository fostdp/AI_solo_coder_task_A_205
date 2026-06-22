FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /sail-simulation .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /sail-simulation /sail-simulation

ENV TZ=Asia/Shanghai
ENV DATABASE_URL=postgres://postgres:postgres@timescaledb:5432/sail_simulation?sslmode=disable
ENV UDP_ADDR=:8001
ENV HTTP_ADDR=:8080
ENV MQTT_BROKER=tcp://mosquitto:1883
ENV CONFIG_PATH=/config
ENV FRONTEND_DIR=/frontend

EXPOSE 8080 8001/udp 9090

ENTRYPOINT ["/sail-simulation"]
