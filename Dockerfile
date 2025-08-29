# Pin Go Alpine image with digest for security
FROM golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052 AS builder

# Ensure Go can auto-install the required toolchain from go.mod (go1.23.x)
ENV GOTOOLCHAIN=auto

# Install build dependencies (avoid strict pins to support Alpine updates)
RUN apk add --no-cache --update \
    build-base \
    sqlite-dev

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
FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9f88e0d99c1ceedbce565fad7d3f96744d15e6919c19c7dafe84a6dd9a80c61

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

# Copy statically linked binary (distroless doesn't have shell/package manager)
COPY --from=builder --chown=nonroot:nonroot /app/whatsignal /app/whatsignal

# Expose port (non-privileged port)
EXPOSE 8082

# Security: Use non-root user (distroless nonroot user: uid=65532)
USER nonroot:nonroot

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