# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files (if they exist)
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxy .

# Production stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 appuser && \
    adduser -D -u 1001 -G appuser appuser

# Copy binary from builder
COPY --from=builder /app/proxy .

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 4848

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:4848/ || exit 1

# Run the proxy
CMD ["./proxy"]
