# Build stage
FROM golang:1.22-bullseye AS builder

# Update system and install dependencies
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./
COPY go.sum* ./

# Download dependencies (if any)
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/reai ./cmd/server

# Final stage
FROM debian:bullseye-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -u 1001 -m reai

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/bin/reai .

# Create data directory with proper permissions
RUN mkdir -p /app/data && \
    chown -R reai:reai /app && \
    chmod 755 /app/data

# Switch to non-root user
USER reai

# Expose the port
EXPOSE 8080

# Set environment variables with defaults
ENV PORT=8080
ENV DATA_DIR=/app/data
ENV LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

# Run the application
CMD ["./reai"]
