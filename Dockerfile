# ---------- Build stage ----------
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install goose into a known location.
ENV GOBIN=/app/bin
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Cache Go module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build the binaries (static, no CGO needed for pgx).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/seed ./cmd/seed

# ---------- Runtime stage ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates postgresql16-client

WORKDIR /app

COPY --from=builder /app/bin/goose /usr/local/bin/goose
COPY --from=builder /app/bin/api   /app/api
COPY --from=builder /app/bin/seed  /app/seed
COPY migrations /app/migrations
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
