# Build Frontend
FROM oven/bun:latest AS frontend-builder
WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Build Backend
FROM golang:1.25.5-alpine AS backend-builder
RUN apk add --no-cache build-base git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist
RUN GIT_VERSION=$(git describe --tags --always --dirty --match "v*" | sed 's/-g\([0-9a-f]\{7,\}\)/-\1/') && \
    GIT_COMMIT=$(git rev-parse --short HEAD) && \
    BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    go build -ldflags="-X main.Version=${GIT_VERSION} -X main.Commit=${GIT_COMMIT} -X main.Date=${BUILD_DATE}" -o /app/urfd-dashboard ./cmd/dashboard

# Final Stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/urfd-dashboard .
EXPOSE 8080
ENTRYPOINT ["./urfd-dashboard"]
