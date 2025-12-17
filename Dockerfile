# =============================================================================
# Study in Woods - Unified Multi-Service Dockerfile
# =============================================================================
# This Dockerfile builds all services in the turborepo monorepo:
# 1. Next.js Web App (apps/web) - Port 3000
# 2. Go Fiber API (apps/api) - Port 8080
# 3. Python OCR Service (apps/ocr-service) - Port 8081
#
# Build targets:
#   - web: Next.js frontend only
#   - api: Go backend only
#   - ocr: Python OCR service only
#   - all: All services (default)
#
# Usage:
#   docker build --target web -t study-woods-web .
#   docker build --target api -t study-woods-api .
#   docker build --target ocr -t study-woods-ocr .
#   docker build -t study-woods .
# =============================================================================

# =============================================================================
# Stage 1: Base Node.js image for web app
# =============================================================================
FROM node:20-alpine AS web-base
RUN apk add --no-cache libc6-compat
WORKDIR /app

# Install turbo globally
RUN npm install -g turbo

# =============================================================================
# Stage 2: Web App Dependencies
# =============================================================================
FROM web-base AS web-deps
WORKDIR /app

# Copy root package files
COPY package.json package-lock.json turbo.json ./
COPY apps/web/package.json ./apps/web/

# Install dependencies (hoisted to root node_modules)
RUN npm ci --legacy-peer-deps

# =============================================================================
# Stage 3: Web App Builder
# =============================================================================
FROM web-base AS web-builder
WORKDIR /app

# Copy dependencies (only root node_modules exists with npm workspaces)
COPY --from=web-deps /app/node_modules ./node_modules

# Copy source files
COPY package.json package-lock.json turbo.json ./
COPY apps/web ./apps/web

# Build the web app
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

# Build-time environment variables for Next.js
ARG NEXT_PUBLIC_API_URL=https://api.studyinwoods.app
ARG NEXT_PUBLIC_APP_URL=https://studyinwoods.app
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL

RUN npm run web:build

# =============================================================================
# Stage 4: Web App Production (HARDENED)
# =============================================================================
FROM node:20-alpine AS web
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Create non-root user
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# SECURITY: Remove attack tools that can be used for reverse shells
# Remove wget, nc (netcat), curl, and other potentially dangerous binaries
RUN apk del wget curl busybox-extras 2>/dev/null || true && \
    rm -f /usr/bin/wget /usr/bin/nc /usr/bin/curl /usr/bin/ncat 2>/dev/null || true && \
    rm -rf /tmp/* /var/tmp/* && \
    chmod 1733 /tmp

# Copy built assets
# Note: public folder must be at apps/web/public (relative to server.js location)
COPY --from=web-builder /app/apps/web/public ./apps/web/public
COPY --from=web-builder --chown=nextjs:nodejs /app/apps/web/.next/standalone ./
COPY --from=web-builder --chown=nextjs:nodejs /app/apps/web/.next/static ./apps/web/.next/static

USER nextjs

EXPOSE 3000

# SECURITY: Use node for healthcheck instead of wget
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD node -e "require('http').get('http://127.0.0.1:3000/', (r) => process.exit(r.statusCode === 200 ? 0 : 1)).on('error', () => process.exit(1))"

CMD ["node", "apps/web/server.js"]

# =============================================================================
# Stage 5: Go API Builder
# =============================================================================
FROM golang:1.24-alpine AS api-builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make ca-certificates tzdata

# Copy go mod files
COPY apps/api/go.mod apps/api/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY apps/api/ ./

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(date +%Y%m%d)" \
    -o /app/server .

# =============================================================================
# Stage 6: Go API Production
# =============================================================================
FROM alpine:3.19 AS api
WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder
COPY --from=api-builder /app/server .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./server"]

# =============================================================================
# Stage 7: Python OCR Service Builder
# =============================================================================
FROM python:3.11-slim AS ocr-builder
WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY apps/ocr-service/requirements.txt .

# Build wheels
RUN pip install --no-cache-dir --user -r requirements.txt

# =============================================================================
# Stage 8: Python OCR Service Production
# =============================================================================
FROM python:3.11-slim AS ocr
WORKDIR /app

# Install runtime dependencies (libgomp for numpy/scipy)
RUN apt-get update && apt-get install -y \
    libgomp1 \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user first
RUN useradd -m -u 1000 ocruser

# Copy Python packages from builder to user's home
COPY --from=ocr-builder /root/.local /home/ocruser/.local

# Make sure scripts in .local are usable
ENV PATH=/home/ocruser/.local/bin:$PATH

# Copy application code
COPY apps/ocr-service/main.py .
COPY apps/ocr-service/requirements.txt .

# Change ownership
RUN chown -R ocruser:ocruser /app /home/ocruser/.local

USER ocruser

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8081", "--workers", "1"]

# =============================================================================
# Stage 9: Combined All Services (for docker-compose orchestration reference)
# =============================================================================
# Note: In production, run each service as a separate container
# This stage is just a reference/documentation stage
FROM alpine:3.19 AS all
RUN echo "Use docker-compose.prod.yml to run all services together"
RUN echo "Services: web (3000), api (8080), ocr (8081)"
CMD ["echo", "Run individual service targets: web, api, ocr"]
