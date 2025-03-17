# ARN Protocol (AI Registry Node)

ARN is an open protocol specification for AI service discovery and capability advertisement, inspired by foundational internet protocols like DNS and TCP. It provides a standardized way for AI services to discover and communicate with each other, while also bridging with data sources through MCP (Model Context Protocol) integration.

## Core Concepts

- **Decentralized Discovery**: Any AI can join the network and advertise its capabilities
- **Capability Advertisement**: AIs publish their abilities without exposing implementation details
- **Secure Handshaking**: Standardized process for AI-to-AI initial communication
- **Protocol Agnostic**: Independent of specific AI implementations or frameworks
- **MCP Bridge**: Integration with Model Context Protocol for data source access

## Standard Interaction Patterns

1. **DISCOVER** (like DNS A record lookup)
   - Find AI services by capability
   - Returns endpoint and interaction requirements

2. **NEGOTIATE** (like TLS handshake)
   - Establish secure connection
   - Agree on protocol version
   - Exchange capability requirements

3. **STREAM** (like WebSocket)
   - Continuous AI-to-AI communication
   - Real-time data exchange
   - State synchronization

4. **DELEGATE** (like DNS CNAME)
   - Forward requests to specialized AIs
   - Chain AI capabilities
   - Load balancing

## Error Code Standards

```
1xx: Protocol Errors
- 100: Invalid Version
- 101: Invalid Message Type
- 102: Invalid Payload

2xx: Authentication/Authorization
- 200: Unauthorized
- 201: Forbidden
- 202: Invalid Credentials

3xx: Capability Errors
- 300: Capability Not Found
- 301: Capability Unavailable
- 302: Invalid Capability Format

4xx: MCP Bridge Errors
- 400: MCP Endpoint Unavailable
- 401: MCP Protocol Mismatch
- 402: MCP Authentication Failed
```

## Project Structure

```
arn-protocol/
├── cmd/                    # Command-line tools
│   └── server/            # ARN server implementation
└── pkg/                    # Public packages
    ├── protocol/           # Core protocol implementation
    │   ├── types.go       # Protocol types and constants
    │   └── handler.go     # Protocol message handling
    └── network/           # Network layer
        └── server.go      # TCP/UDP server implementation
```

## MCP Integration

ARN works alongside MCP (Model Context Protocol) to provide a complete ecosystem:
- ARN handles service discovery and AI-to-AI communication
- MCP handles data system to AI communication
- ARN can advertise data availability through MCP endpoints

This creates a complete ecosystem where:
1. AIs can discover other AIs (via ARN)
2. AIs can discover data sources (via ARN->MCP bridge)
3. Data systems can communicate with AIs (via MCP)

## Getting Started

### Prerequisites
```bash
# Install Go 1.21 or later
brew install go
```

### Running an ARN Node
```go
package main

import (
    "github.com/heathweaver/arn-protocol/pkg/network"
    "github.com/heathweaver/arn-protocol/pkg/protocol"
)

func main() {
    handler := protocol.NewHandler(nil, nil)
    server := network.NewServer(":7777", ":7778", handler)
    server.Start()
}
```

### Registering an AI Service
```go
cap := &protocol.Capability{
    ID:   "my-ai-service",
    Name: "Example AI Service",
    Type: "DISCOVER",
    MCPEnabled: true,  // Can use MCP data sources
}

handler.RegisterCapability(cap)
```

## Contributing

ARN is an open protocol. Contributions to the specification are welcome through the standard RFC process. We follow semantic versioning and maintain backward compatibility.

## License

MIT
