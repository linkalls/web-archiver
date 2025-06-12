# Stage 1: Builder
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Install build tools
RUN apk add --no-cache git

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# Using CGO_ENABLED=0 to build a statically linked binary, which is good for alpine
# Add -ldflags "-s -w" to strip debug symbols and reduce binary size
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /app/archive-lite main.go

# Stage 2: Runtime
FROM alpine:latest

WORKDIR /app

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create data directories and set permissions
# These directories will store the archived content and screenshots
RUN mkdir -p /app/data/raw &&     mkdir -p /app/data/screenshots &&     chown -R appuser:appgroup /app/data

# Copy the compiled binary from the builder stage
COPY --from=builder /app/archive-lite /app/archive-lite

# Ensure the binary is executable
RUN chmod +x /app/archive-lite

# The SQLite database file (archive.db) will be created in /app/ by default
# If a volume is mounted at /app/data, it could be placed there too with config.
# For now, ensure the /app directory is writable by the appuser if db is in /app
RUN chown appuser:appgroup /app

# Switch to the non-root user
USER appuser

# Expose the application port
EXPOSE 3000

# Set the entry point
ENTRYPOINT ["/app/archive-lite"]

# Default command (can be overridden)
# CMD []
