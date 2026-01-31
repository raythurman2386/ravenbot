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

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o ravenbot ./cmd/bot/main.go

# Final stage
FROM alpine:latest

# Install certificates, timezone data, Chromium for headless browsing, and Node.js for MCP servers
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont \
    nodejs \
    npm \
    git

# Pre-install MCP servers for performance
RUN npm install -g @modelcontextprotocol/server-filesystem @cyanheads/git-mcp-server @modelcontextprotocol/server-github @modelcontextprotocol/server-memory

# Set Chrome path for chromedp
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROMEDP_NO_SANDBOX=true

WORKDIR /app

# Copy binary and config from builder
COPY --from=builder /app/ravenbot .
COPY --from=builder /app/config.json* ./

# Create directory for logs
RUN mkdir -p daily_logs

# Use a non-root user (optional but recommended)
# RUN adduser -D ravenuser
# USER ravenuser

CMD ["./ravenbot"]
