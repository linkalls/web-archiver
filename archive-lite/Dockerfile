# Stage 1: Builder
FROM golang:1.23-bullseye AS builder

WORKDIR /src

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags "-s -w" -o /app/archive-lite main.go

# Stage 2: Runtime
FROM debian:bullseye-slim

WORKDIR /app

# Install basic dependencies
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user and group
RUN groupadd -r appgroup && useradd --no-log-init -r -g appgroup -d /app appuser

# Create data directories and set permissions
RUN mkdir -p /app/data/raw && chown -R appuser:appgroup /app/data

# Copy the compiled binary from the builder stage
COPY --from=builder /app/archive-lite /app/archive-lite
COPY webui.html /app/webui.html

# Ensure the binary is executable
RUN chmod +x /app/archive-lite

# Ensure /app is writable by appuser
RUN chown appuser:appgroup /app

# Switch to the non-root user
USER appuser

EXPOSE 3000
ENTRYPOINT ["/app/archive-lite"]
