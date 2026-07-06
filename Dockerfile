ARG GO_VERSION=1.25.8

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN go mod tidy \
    && go mod download \
    && CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.Name=aisphere-iam -X main.Version=${VERSION}" \
      -o /app/server \
      ./cmd/aisphere-iam

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=builder /src/configs /app/configs
COPY --from=builder /src/migrations /app/migrations
RUN chown -R app:app /app

USER app
EXPOSE 18080 19080 19180

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:18080/healthz || exit 1

CMD ["./server", "-conf", "./configs/config.yaml"]
