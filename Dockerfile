FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o whatsignal ./cmd/whatsignal

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /app
COPY --from=builder /app/whatsignal .

RUN mkdir -p /app/cache /app/data

EXPOSE 8082

CMD ["/app/whatsignal"] 