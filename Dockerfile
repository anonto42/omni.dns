# Stage 1: Build React frontend
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Build Go server
FROM golang:1.22-alpine AS go-builder
WORKDIR /server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=0 go build -o /dns-server ./cmd/dns-server

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -h /app dns
WORKDIR /app

COPY --from=go-builder /dns-server .
COPY --from=web-builder /web/dist ./web/dist

EXPOSE 53 53/udp 8080

USER dns
ENTRYPOINT ["/app/dns-server"]
CMD ["--static", "web/dist"]
