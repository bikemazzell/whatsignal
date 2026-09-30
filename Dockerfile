# Use latest Alpine Go image for smaller size
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

# Ensure Go can auto-install the required toolchain from go.mod
ENV GOTOOLCHAIN=auto

# Install build dependencies with proper error handling
RUN set -eux; \
    # Use the base image's own repositories; overriding them pins a different
    # Alpine release than the base and mismatches musl/toolchain versions.
    apk update; \
    # Install required packages
    apk add --no-cache \
        build-base \
        sqlite-dev \
        ca-certificates

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

# Final stage - use distroless image for security
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

# Add security labels
LABEL org.opencontainers.image.title="WhatsSignal" \
      org.opencontainers.image.description="Secure WhatsApp-Signal Bridge" \
      org.opencontainers.image.vendor="WhatsSignal Project" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/user/whatsignal" \
      security.non-root="true" \
      security.read-only-root="true" \
      security.no-shell="true"

WORKDIR /app

# Copy statically linked binary and migrations (distroless doesn't have shell/package manager)
COPY --from=builder --chown=nonroot:nonroot /app/whatsignal /app/whatsignal
COPY --from=builder --chown=nonroot:nonroot /app/scripts/migrations /app/scripts/migrations

# Expose port (non-privileged port)
EXPOSE 8082

# Security: Use non-root user (distroless nonroot user: uid=65532)
USER nonroot:nonroot

# Health check using the binary's built-in --healthcheck flag (distroless has no shell/curl)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/app/whatsignal", "--healthcheck"]

# Default command with explicit path
ENTRYPOINT ["/app/whatsignal"]

# Document required volumes for data persistence
# These should be mounted as volumes in production:
# - /app/data (database and persistent storage)
# - /app/media-cache (media file cache)
# - /app/signal-attachments (signal attachment storage)
#
# Example docker run with proper volumes:
# docker run -d \
#   --read-only \
#   --tmpfs /tmp:noexec,nosuid,size=100m \
#   --tmpfs /var/tmp:noexec,nosuid,size=100m \
#   -v whatsignal-data:/app/data:rw \
#   -v whatsignal-cache:/app/media-cache:rw \
#   -v whatsignal-attachments:/app/signal-attachments:rw \
#   --cap-drop ALL \
#   --security-opt no-new-privileges=true \
#   -p 8082:8082 \
#   whatsignal:latest
