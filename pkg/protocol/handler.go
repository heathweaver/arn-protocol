package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Handler manages protocol communication
type Handler struct {
	capabilities     map[string]*Capability
	mcpBridges      map[string]*MCPBridge
	mu              sync.RWMutex
	onMessage       func(*Message) error
	onMCPBridge     func(*MCPBridge) error
}

// MCPBridge represents a bridge to an MCP data source
type MCPBridge struct {
	ID          string            `json:"id"`
	Endpoint    string            `json:"endpoint"`
	Protocol    string            `json:"protocol"` // MCP protocol version
	DataTypes   []string          `json:"data_types"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	LastUpdated time.Time         `json:"last_updated"`
}

// NewHandler creates a new protocol handler
func NewHandler(onMessage func(*Message) error, onMCPBridge func(*MCPBridge) error) *Handler {
	return &Handler{
		capabilities: make(map[string]*Capability),
		mcpBridges:  make(map[string]*MCPBridge),
		onMessage:   onMessage,
		onMCPBridge: onMCPBridge,
	}
}

// RegisterCapability registers an AI capability
func (h *Handler) RegisterCapability(cap *Capability) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if cap.ID == "" {
		return fmt.Errorf("capability ID required")
	}

	h.capabilities[cap.ID] = cap
	return nil
}

// RegisterMCPBridge registers an MCP data source bridge
func (h *Handler) RegisterMCPBridge(bridge *MCPBridge) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if bridge.ID == "" {
		return fmt.Errorf("bridge ID required")
	}

	h.mcpBridges[bridge.ID] = bridge
	
	// Notify about new MCP bridge if handler exists
	if h.onMCPBridge != nil {
		if err := h.onMCPBridge(bridge); err != nil {
			return fmt.Errorf("mcp bridge notification failed: %w", err)
		}
	}

	return nil
}

// HandleMessage processes an incoming message
func (h *Handler) HandleMessage(ctx context.Context, msg *Message) (*Message, error) {
	switch msg.Type {
	case Hello:
		return h.handleHello(msg)
	case Register:
		return h.handleRegister(msg)
	case Query:
		return h.handleQuery(msg)
	case MCPBridgeAdvertise:
		return h.handleMCPBridgeAdvertise(msg)
	case MCPBridgeRequest:
		return h.handleMCPBridgeRequest(msg)
	default:
		if h.onMessage != nil {
			if err := h.onMessage(msg); err != nil {
				return nil, fmt.Errorf("message handler error: %w", err)
			}
		}
		return nil, nil
	}
}

func (h *Handler) handleHello(msg *Message) (*Message, error) {
	// Simple hello response with protocol version
	response := &Message{
		Version:   V1,
		Type:      Hello,
		Timestamp: time.Now(),
	}
	return response, nil
}

func (h *Handler) handleRegister(msg *Message) (*Message, error) {
	var cap Capability
	if err := json.Unmarshal(msg.Payload, &cap); err != nil {
		return createErrorMessage(ErrInvalidPayload, "invalid capability format")
	}

	if err := h.RegisterCapability(&cap); err != nil {
		return createErrorMessage(ErrInvalidCapabilityFormat, err.Error())
	}

	response := &Message{
		Version:   V1,
		Type:      Response,
		Timestamp: time.Now(),
	}
	return response, nil
}

func (h *Handler) handleQuery(msg *Message) (*Message, error) {
	var query struct {
		CapabilityType string `json:"capability_type"`
		MCPEnabled     bool   `json:"mcp_enabled,omitempty"`
	}

	if err := json.Unmarshal(msg.Payload, &query); err != nil {
		return createErrorMessage(ErrInvalidPayload, "invalid query format")
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Filter capabilities based on query
	matches := make([]*Capability, 0)
	for _, cap := range h.capabilities {
		if cap.Type == query.CapabilityType && (!query.MCPEnabled || cap.MCPEnabled) {
			matches = append(matches, cap)
		}
	}

	// Prepare response
	payload, err := json.Marshal(matches)
	if err != nil {
		return createErrorMessage(ErrInvalidPayload, "failed to marshal response")
	}

	return &Message{
		Version:   V1,
		Type:      Response,
		Payload:   payload,
		Timestamp: time.Now(),
	}, nil
}

func (h *Handler) handleMCPBridgeAdvertise(msg *Message) (*Message, error) {
	var bridge MCPBridge
	if err := json.Unmarshal(msg.Payload, &bridge); err != nil {
		return createErrorMessage(ErrInvalidPayload, "invalid MCP bridge format")
	}

	if err := h.RegisterMCPBridge(&bridge); err != nil {
		return createErrorMessage(ErrMCPEndpointUnavailable, err.Error())
	}

	response := &Message{
		Version:   V1,
		Type:      Response,
		Timestamp: time.Now(),
	}
	return response, nil
}

func (h *Handler) handleMCPBridgeRequest(msg *Message) (*Message, error) {
	var request struct {
		BridgeID  string `json:"bridge_id"`
		DataType  string `json:"data_type"`
	}

	if err := json.Unmarshal(msg.Payload, &request); err != nil {
		return createErrorMessage(ErrInvalidPayload, "invalid bridge request format")
	}

	h.mu.RLock()
	bridge, exists := h.mcpBridges[request.BridgeID]
	h.mu.RUnlock()

	if !exists {
		return createErrorMessage(ErrMCPEndpointUnavailable, "bridge not found")
	}

	// Check if requested data type is supported
	supported := false
	for _, dt := range bridge.DataTypes {
		if dt == request.DataType {
			supported = true
			break
		}
	}

	if !supported {
		return createErrorMessage(ErrMCPProtocolMismatch, "unsupported data type")
	}

	// Return bridge details
	payload, err := json.Marshal(bridge)
	if err != nil {
		return createErrorMessage(ErrInvalidPayload, "failed to marshal bridge details")
	}

	return &Message{
		Version:   V1,
		Type:      MCPBridgeResponse,
		Payload:   payload,
		Timestamp: time.Now(),
	}, nil
}

func createErrorMessage(code ErrorCode, message string) (*Message, error) {
	errPayload := struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
	}{
		Code:    code,
		Message: message,
	}

	payload, err := json.Marshal(errPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create error message: %w", err)
	}

	return &Message{
		Version:   V1,
		Type:      Error,
		Payload:   payload,
		Timestamp: time.Now(),
	}, nil
}
