# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/bin/keylightd /usr/local/bin/

# Create non-root user
RUN adduser -D -g '' keylightd

# Switch to non-root user
USER keylightd

# Expose ports
EXPOSE 8080

# Run the application
ENTRYPOINT ["keylightd"] 