package protocol

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Version represents the ARN protocol version
type Version uint8

const (
	V1 Version = 1
)

// MessageType represents different types of ARN messages
type MessageType uint8

const (
	// Core message types
	Hello MessageType = iota + 1
	Register
	Query
	Response
	Handshake
	Error

	// AI-to-AI specific messages
	AICapabilityAdvertise
	AICapabilityRequest
	AIStreamStart
	AIStreamData
	AIStreamEnd

	// MCP bridge messages
	MCPBridgeAdvertise  // Advertise MCP data source
	MCPBridgeRequest    // Request access to MCP data
	MCPBridgeResponse   // Response with MCP endpoint details
)

// ErrorCode represents standardized error codes
type ErrorCode uint16

const (
	// 1xx: Protocol errors
	ErrInvalidVersion ErrorCode = 100 + iota
	ErrInvalidMessageType
	ErrInvalidPayload

	// 2xx: Authentication/Authorization errors
	ErrUnauthorized = 200 + iota
	ErrForbidden
	ErrInvalidCredentials

	// 3xx: Capability errors
	ErrCapabilityNotFound = 300 + iota
	ErrCapabilityUnavailable
	ErrInvalidCapabilityFormat

	// 4xx: MCP bridge errors
	ErrMCPEndpointUnavailable = 400 + iota
	ErrMCPProtocolMismatch
	ErrMCPAuthenticationFailed
)

// InteractionType represents different ways AIs can interact
type InteractionType uint8

const (
	// Core interaction patterns
	Discover InteractionType = iota + 1
	Negotiate
	Stream
	Delegate
)

// Capability represents an AI's capability or a data source's capability
type Capability struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Version     string            `json:"version"`
	Interaction InteractionType   `json:"interaction"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	MCPEnabled  bool             `json:"mcp_enabled,omitempty"` // Whether this capability can interact via MCP
}

// Message represents the base ARN message format
type Message struct {
	Version     Version
	Type        MessageType
	PayloadSize uint32
	Payload     []byte
	Timestamp   time.Time
}

// Serialize converts a Message to its wire format
func (m *Message) Serialize() ([]byte, error) {
	if len(m.Payload) > 1<<32-1 {
		return nil, fmt.Errorf("payload too large")
	}

	// Calculate total size: version(1) + type(1) + size(4) + payload + timestamp(8)
	totalSize := 1 + 1 + 4 + len(m.Payload) + 8
	buffer := make([]byte, totalSize)

	// Write version and type
	buffer[0] = byte(m.Version)
	buffer[1] = byte(m.Type)

	// Write payload size
	binary.BigEndian.PutUint32(buffer[2:6], uint32(len(m.Payload)))

	// Write payload
	copy(buffer[6:6+len(m.Payload)], m.Payload)

	// Write timestamp
	binary.BigEndian.PutUint64(buffer[6+len(m.Payload):], uint64(m.Timestamp.UnixNano()))

	return buffer, nil
}

// Deserialize converts wire format back to a Message
func Deserialize(data []byte) (*Message, error) {
	if len(data) < 14 { // Minimum size: version(1) + type(1) + size(4) + timestamp(8)
		return nil, fmt.Errorf("message too short")
	}

	msg := &Message{
		Version: Version(data[0]),
		Type:    MessageType(data[1]),
	}

	// Read payload size
	msg.PayloadSize = binary.BigEndian.Uint32(data[2:6])

	// Validate total message size
	expectedSize := 6 + msg.PayloadSize + 8
	if uint32(len(data)) != expectedSize {
		return nil, fmt.Errorf("invalid message size")
	}

	// Read payload
	msg.Payload = make([]byte, msg.PayloadSize)
	copy(msg.Payload, data[6:6+msg.PayloadSize])

	// Read timestamp
	nsec := binary.BigEndian.Uint64(data[6+msg.PayloadSize:])
	msg.Timestamp = time.Unix(0, int64(nsec))

	return msg, nil
}
