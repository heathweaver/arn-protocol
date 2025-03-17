FROM golang:1.21-alpine

WORKDIR /app

# Copy only the protocol implementation
COPY pkg/protocol /app/pkg/protocol
COPY pkg/network /app/pkg/network
COPY cmd/server /app/cmd/server

# Build the server
RUN CGO_ENABLED=0 GOOS=linux go build -o /arn-server ./cmd/server

# Expose TCP and UDP ports
EXPOSE 7777/tcp
EXPOSE 7778/udp

# Run the binary
CMD ["/arn-server"]
