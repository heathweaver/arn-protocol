# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /arn-server ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /

# Copy the binary from builder
COPY --from=builder /arn-server /arn-server

# Create non-root user
RUN adduser -D -g '' arn
USER arn

# Expose TCP and UDP ports
EXPOSE 7777/tcp
EXPOSE 7778/udp

# Run the binary
ENTRYPOINT ["/arn-server"]
