# Development stage (hot reload)
FROM golang:1.22-alpine AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
EXPOSE 8080 53 53/udp
CMD ["air", "-c", ".air.toml"]

# Stage 1: Build React frontend
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY frontend/apps/web-app/package.json frontend/apps/web-app/package-lock.json ./
RUN npm ci
COPY frontend/apps/web-app/ .
RUN npm run build

# Stage 2: Build Go server
FROM golang:1.22-alpine AS go-builder
WORKDIR /server
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -o /dns-server ./cmd/dns-server

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -h /app dns
WORKDIR /app

COPY --from=go-builder /dns-server .
COPY --from=web-builder /web/dist ./static

EXPOSE 53 53/udp 8080

USER dns
ENTRYPOINT ["/app/dns-server"]
CMD ["--static", "static"]
