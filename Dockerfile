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

# Install certificates, timezone data, Chromium for headless browsing, Node.js for MCP servers,
# and development/diagnostic tools (Docker, Go, Build tools, Curl)
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
    git \
    docker-cli \
    go \
    build-base \
    curl \
    procps

# Pre-install MCP servers for performance
RUN npm install -g @modelcontextprotocol/server-filesystem \
    @cyanheads/git-mcp-server \
    @modelcontextprotocol/server-github \
    @modelcontextprotocol/server-memory \
    @modelcontextprotocol/server-sequential-thinking

# Set Chrome path for chromedp
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROMEDP_NO_SANDBOX=true

WORKDIR /app

# Copy binary and config from builder
COPY --from=builder /app/ravenbot .
COPY --from=builder /app/config.json* ./

# Create directory for logs
RUN mkdir -p daily_logs

# Setup docker permissions and user
RUN addgroup -g 1001 docker && \
    adduser -D ravenuser && \
    addgroup ravenuser docker

USER ravenuser

CMD ["/app/ravenbot"]
