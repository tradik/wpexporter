# Multi-stage build for WordPress Export JSON
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

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

# Build the applications for target platform.
#
# The linker stamps internal/version, the one package every command reads its
# identity from; an unstamped image would report whatever default that package
# carries. GitCommit stays unset because .dockerignore excludes .git.
RUN VERSION="v$(tr -d '[:space:]' < VERSION)" && \
    VERSION_PKG="github.com/tradik/wpexporter/internal/version" && \
    LDFLAGS="-X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -s -w" && \
    for cmd in wpexportjson wpxmlrpc wpmcp wpexporter; do \
        CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
            go build -ldflags "${LDFLAGS}" -o "${cmd}" "./cmd/${cmd}" || exit 1; \
    done

# Final stage
FROM alpine:3.24

# Install ca-certificates for HTTPS requests and create non-root user
RUN apk add --no-cache --no-scripts ca-certificates && \
    addgroup -g 1001 -S wpexport && \
    adduser -u 1001 -S wpexport -G wpexport

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/wpexportjson /app/wpxmlrpc /app/wpmcp /app/wpexporter /usr/local/bin/

# Copy configuration example
COPY config.example.yaml /app/

# Create export directory
RUN mkdir -p /app/export && chown -R wpexport:wpexport /app

# Switch to non-root user
USER wpexport

# Set default command
CMD ["wpexporter", "--help"]
