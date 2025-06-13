# Stage 1: Builder
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Install build tools (git needed for go mod download private repos, ca-certificates for HTTPS)
RUN apk add --no-cache git ca-certificates

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /app/archive-lite main.go

# Stage 2: Runtime
FROM debian:bullseye-slim

WORKDIR /app

# Install dependencies for Chrome
# Based on https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#chrome-headless-doesnt-launch-on-unix
# and https://www.google.com/linuxrepositories/
RUN apt-get update     && apt-get install -y --no-install-recommends     ca-certificates     fonts-liberation     libasound2     libatk-bridge2.0-0     libatk1.0-0     libc6     libcairo2     libcups2     libdbus-1-3     libexpat1     libfontconfig1     libgbm1     libgcc1     libglib2.0-0     libgtk-3-0     libnspr4     libnss3     libpango-1.0-0     libpangocairo-1.0-0     libstdc++6     libx11-6     libx11-xcb1     libxcb1     libxcomposite1     libxcursor1     libxdamage1     libxext6     libxfixes3     libxi6     libxrandr2     libxrender1     libxss1     libxtst6     lsb-release     wget     xdg-utils     # Install Google Chrome Stable
    && wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb     && apt-get install -y ./google-chrome-stable_current_amd64.deb     && rm google-chrome-stable_current_amd64.deb     # Clean up
    && apt-get clean     && rm -rf /var/lib/apt/lists/*

# Create a non-root user and group
RUN groupadd -r appgroup && useradd --no-log-init -r -g appgroup -d /app appuser

# Create data directories and set permissions
RUN mkdir -p /app/data/raw &&     mkdir -p /app/data/screenshots &&     chown -R appuser:appgroup /app/data

# Copy the compiled binary from the builder stage
COPY --from=builder /app/archive-lite /app/archive-lite

# Ensure the binary is executable
RUN chmod +x /app/archive-lite

# Ensure /app is writable by appuser (e.g. for SQLite DB if not using /app/data)
RUN chown appuser:appgroup /app

# Switch to the non-root user
USER appuser

# Optional: Set CHROME_BIN_PATH if Chrome is not in the default PATH for some reason.
# Google Chrome stable is typically installed to /usr/bin/google-chrome-stable
# ENV CHROME_BIN_PATH=/usr/bin/google-chrome-stable

EXPOSE 3000
ENTRYPOINT [/app/archive-lite]
