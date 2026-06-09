FROM node:22-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
COPY backend .
ENV CGO_ENABLED=1
RUN go build -o dns-server ./cmd/dns-server

FROM golang:1.25-alpine AS dev
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
RUN go install github.com/air-verse/air@latest
COPY backend .
ENV CGO_ENABLED=1
CMD ["air", "-c", ".air.toml"]

FROM golang:1.25-alpine AS test
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
ENV CGO_ENABLED=1
CMD ["sh", "-c", "golangci-lint run ./... && go test ./..."]

FROM alpine:latest AS production
RUN addgroup -S app && adduser -S -G app app
RUN apk add --no-cache libcap
WORKDIR /app
EXPOSE 53/udp 8080/tcp
COPY --from=builder /app/dns-server .
RUN setcap cap_net_bind_service=+ep /app/dns-server
COPY --from=frontend-builder /app/dist ./web/dist
USER app
CMD ["./dns-server"]
