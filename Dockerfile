FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o whatsignal ./cmd/whatsignal

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite

RUN addgroup -g 1001 -S whatsignal && \
    adduser -u 1001 -S whatsignal -G whatsignal

WORKDIR /app
COPY --from=builder /app/whatsignal .

RUN mkdir -p /app/cache /app/data && \
    chown -R whatsignal:whatsignal /app && \
    chmod 755 /app/whatsignal

USER whatsignal

EXPOSE 8082

CMD ["/app/whatsignal"] 