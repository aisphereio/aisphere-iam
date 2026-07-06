FROM golang:1.26 AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.Name=aisphere-iam -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /app/server \
    ./cmd/aisphere-iam

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/server /app/server
COPY --from=builder /src/configs /app/configs
COPY --from=builder /src/migrations /app/migrations
WORKDIR /app

EXPOSE 18080 19080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:18080/healthz || exit 1

CMD ["./server", "-conf", "./configs/config.yaml"]