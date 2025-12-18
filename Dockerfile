# =============================================================================
# Study in Woods - Backend Services Dockerfile
# =============================================================================
# This Dockerfile builds backend services:
# 1. Go Fiber API (apps/api) - Port 8080
# 2. Python OCR Service (apps/ocr-service) - Port 8081
#
# Frontend is deployed separately on Vercel.
#
# Build targets:
#   - api: Go backend only
#   - ocr: Python OCR service only
#
# Usage:
#   docker build --target api -t study-woods-api .
#   docker build --target ocr -t study-woods-ocr .
# =============================================================================

# =============================================================================
# Stage 1: Go API Builder
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
# Stage 2: Go API Production
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
# Stage 3: Python OCR Service Builder
# =============================================================================
FROM python:3.12-slim AS ocr-builder
WORKDIR /build

# Install build dependencies + poppler for pdf2image
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY apps/ocr-service/requirements.txt .

# Build wheels
RUN pip install --no-cache-dir --user -r requirements.txt

# Pre-download models during build
COPY apps/ocr-service/preload_models.py .
ENV PATH=/root/.local/bin:$PATH
RUN python preload_models.py

# =============================================================================
# Stage 4: Python OCR Service Production
# =============================================================================
FROM python:3.12-slim AS ocr
WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    libgomp1 \
    libgl1 \
    libglib2.0-0 \
    poppler-utils \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user first
RUN useradd -m -u 1000 ocruser

# Copy Python packages from builder to user's home
COPY --from=ocr-builder /root/.local /home/ocruser/.local

# Copy pre-downloaded PaddleOCR models
COPY --from=ocr-builder /root/.paddlex /home/ocruser/.paddlex

# Performance optimization environment variables
ENV OMP_NUM_THREADS=4
ENV MKL_NUM_THREADS=4
ENV PATH=/home/ocruser/.local/bin:$PATH
ENV DISABLE_MODEL_SOURCE_CHECK=True

# Copy application code
COPY apps/ocr-service/main.py .
COPY apps/ocr-service/requirements.txt .

# Change ownership
RUN chown -R ocruser:ocruser /app /home/ocruser/.local /home/ocruser/.paddlex

USER ocruser

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8081", "--workers", "1"]
