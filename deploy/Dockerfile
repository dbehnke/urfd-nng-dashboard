# Build Frontend
FROM oven/bun:latest AS frontend-builder
WORKDIR /app/web
COPY web/package.json web/bun.lockb ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Build Backend
FROM golang:1.25.5-alpine AS backend-builder
RUN apk add --no-cache build-essential
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist
RUN go build -o /app/urfd-dashboard ./cmd/dashboard

# Final Stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/urfd-dashboard .
EXPOSE 8080
ENTRYPOINT ["./urfd-dashboard"]
