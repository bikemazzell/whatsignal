FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy VERSION file
COPY VERSION .

# Build the application with version information
ARG VERSION
ARG BUILD_TIME
ARG GIT_COMMIT
RUN VERSION=${VERSION:-$(cat VERSION)} \
    BUILD_TIME=${BUILD_TIME:-$(date -u '+%Y-%m-%d_%H:%M:%S')} \
    GIT_COMMIT=${GIT_COMMIT:-unknown} \
    CGO_ENABLED=1 GOOS=linux go build -a \
    -ldflags "-extldflags '-static' -X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT" \
    -o whatsignal ./cmd/whatsignal

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite wget

# Create non-root user
RUN addgroup -g 1001 -S whatsignal && \
    adduser -u 1001 -S whatsignal -G whatsignal

WORKDIR /app

# Copy binary and set permissions
COPY --from=builder /app/whatsignal .
RUN chmod 755 /app/whatsignal

# Create directories and set ownership
RUN mkdir -p /app/cache /app/data && \
    chown -R whatsignal:whatsignal /app

# Switch to non-root user
USER whatsignal

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Default command
CMD ["/app/whatsignal"] 