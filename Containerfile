# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o kasa-monitor ./cmd/kasa-monitor

# Runtime stage
FROM alpine:3.20

RUN adduser --home=/srv --shell=/bin/false \
    --disabled-password --no-create-home monitor

WORKDIR /srv
COPY --from=builder /build/kasa-monitor /usr/local/bin/kasa-monitor

USER monitor

ENTRYPOINT ["kasa-monitor"]
CMD ["poll", "-o", "-c", "/etc/kasa-monitor/config.toml"]
