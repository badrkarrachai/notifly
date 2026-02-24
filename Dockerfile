# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o notifly-server cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o notifly-worker cmd/worker/main.go

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/notifly-server .
COPY --from=builder /build/notifly-worker .
COPY --from=builder /build/internal/infra/template/templates /app/templates
COPY --from=builder /build/config.yaml .

# Default to server mode — override with docker-compose command
EXPOSE 8081

CMD ["./notifly-server"]
