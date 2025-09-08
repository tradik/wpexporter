# Multi-stage build for WordPress Export JSON
FROM golang:1.24-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wpexportjson ./cmd/wpexportjson
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wpxmlrpc ./cmd/wpxmlrpc

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S wpexport && \
    adduser -u 1001 -S wpexport -G wpexport

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/wpexportjson /app/wpxmlrpc /usr/local/bin/

# Copy configuration example
COPY config.example.yaml /app/

# Create export directory
RUN mkdir -p /app/export && chown -R wpexport:wpexport /app

# Switch to non-root user
USER wpexport

# Set default command
CMD ["wpexportjson", "--help"]
