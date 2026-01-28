# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy dependencies first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary for ARM64 (Pi 5)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ravenbot ./cmd/bot/main.go

# Final stage
FROM alpine:latest

# Install certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/ravenbot .

# Create directory for logs
RUN mkdir -p daily_logs

# Use a non-root user (optional but recommended)
# RUN adduser -D ravenuser
# USER ravenuser

CMD ["./ravenbot"]
