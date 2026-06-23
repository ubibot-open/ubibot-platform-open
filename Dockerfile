# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ubibot-platform ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 ubibot

WORKDIR /app
COPY --from=builder /out/ubibot-platform /app/ubibot-platform
COPY config.yaml /app/config.yaml

RUN mkdir -p /app/data && chown -R ubibot:ubibot /app
USER ubibot

EXPOSE 8080 1883

ENTRYPOINT ["/app/ubibot-platform"]
