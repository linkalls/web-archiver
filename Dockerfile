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

# Create app user with a specific UID (can be overridden at build time)
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd -g ${GROUP_ID} appuser \
    && useradd -u ${USER_ID} -g appuser -s /bin/bash -m appuser

# Copy the compiled binary from the builder stage
COPY --from=builder /app/archive-lite /app/archive-lite
COPY webui.html /app/webui.html

# Create data directories
RUN mkdir -p /app/data/raw /app/data/assets

# Set ownership to the app user
RUN chown -R appuser:appuser /app

# Ensure the binary is executable
RUN chmod +x /app/archive-lite

# Create entrypoint script
RUN echo '#!/bin/bash\n\
# Ensure data directories exist\n\
mkdir -p /app/data/raw /app/data/assets\n\
\n\
# If USER_ID or GROUP_ID environment variables are set, update the user\n\
if [ ! -z "$USER_ID" ] && [ "$USER_ID" != "1000" ]; then\n\
    usermod -u $USER_ID appuser\n\
fi\n\
if [ ! -z "$GROUP_ID" ] && [ "$GROUP_ID" != "1000" ]; then\n\
    groupmod -g $GROUP_ID appuser\n\
fi\n\
\n\
# Fix ownership after potential user changes\n\
chown -R appuser:appuser /app/data\n\
\n\
# Switch to app user and execute the main application\n\
exec gosu appuser /app/archive-lite\n\
' > /app/entrypoint.sh && chmod +x /app/entrypoint.sh

# Install gosu for proper user switching
RUN apt-get update \
    && apt-get install -y --no-install-recommends gosu \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

EXPOSE 3000
ENTRYPOINT ["/app/entrypoint.sh"]
