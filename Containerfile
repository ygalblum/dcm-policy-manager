# Stage 1: Builder
FROM registry.access.redhat.com/ubi9/go-toolset:1.25.5 AS builder

# Set working directory
WORKDIR /opt/app-root/src

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o policy-manager ./cmd/policy-manager

# Stage 2: Runtime
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /opt/app-root/src/policy-manager .

# Run as non-root user
USER 1001

# Expose port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/app/policy-manager"]
