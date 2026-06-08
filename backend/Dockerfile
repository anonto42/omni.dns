FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o dns-server ./cmd/dns-server

FROM golang:1.24-alpine AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@v1.61.1
COPY . .
CMD ["air", "-c", ".air.toml"]

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/dns-server .
EXPOSE 53/udp 8080/tcp
CMD ["./dns-server"]
