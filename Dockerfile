# Multi-stage build for WordPress Export JSON
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

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

# Build the applications for target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o wpexportjson ./cmd/wpexportjson
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o wpxmlrpc ./cmd/wpxmlrpc

# Final stage
FROM alpine:3.21

# Install ca-certificates for HTTPS requests and create non-root user
RUN apk add --no-cache --no-scripts ca-certificates && \
    addgroup -g 1001 -S wpexport && \
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
