# ============================================
# JCP Web Server - Multi-stage Dockerfile
# ============================================

# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files first for caching
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci --prefer-offline --no-audit

# Copy source code
COPY frontend/ ./

# Build frontend
RUN npm run build

# Stage 2: Build Go Backend
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend to embed location
COPY --from=frontend-builder /app/frontend/dist ./cmd/server/static

# Build the web server binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/jcp-server \
    ./cmd/server

# Stage 3: Final Runtime Image
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=backend-builder /app/jcp-server /app/jcp-server

# Copy embedded stock data (if exists)
COPY --from=backend-builder /app/internal/embed /app/internal/embed

# Create data directory
RUN mkdir -p /app/data && chown -R nobody:nobody /app

# Use non-root user
USER nobody

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

# Set environment variables
ENV PORT=8080
ENV JCP_DATA_DIR=/app/data
ENV TZ=Asia/Shanghai

# Run the server
ENTRYPOINT ["/app/jcp-server"]
