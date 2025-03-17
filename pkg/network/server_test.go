package network

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/heathweaver/arn-protocol/pkg/protocol"
)

func TestTCPServer(t *testing.T) {
	// Create handler with test callbacks
	messageReceived := make(chan *protocol.Message, 1)
	bridgeReceived := make(chan *protocol.MCPBridge, 1)

	handler := protocol.NewHandler(
		func(msg *protocol.Message) error {
			messageReceived <- msg
			return nil
		},
		func(bridge *protocol.MCPBridge) error {
			bridgeReceived <- bridge
			return nil
		},
	)

	// Start server
	server := NewServer("127.0.0.1:0", "127.0.0.1:0", handler)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get actual TCP address
	tcpAddr := server.tcpListener.Addr().String()

	// Test cases
	tests := []struct {
		name    string
		message *protocol.Message
		wantErr bool
	}{
		{
			name: "hello message",
			message: &protocol.Message{
				Version:   protocol.V1,
				Type:     protocol.Hello,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "capability registration",
			message: &protocol.Message{
				Version: protocol.V1,
				Type:    protocol.Register,
				Payload: mustMarshal(t, &protocol.Capability{
					ID:          "test-cap",
					Name:        "Test Capability",
					Type:        "DISCOVER",
					Interaction: protocol.Discover,
				}),
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "mcp bridge advertisement",
			message: &protocol.Message{
				Version: protocol.V1,
				Type:    protocol.MCPBridgeAdvertise,
				Payload: mustMarshal(t, &protocol.MCPBridge{
					ID:       "test-bridge",
					Endpoint: "mcp://test.endpoint",
					Protocol: "MCP/1.0",
					DataTypes: []string{"test_data"},
				}),
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Connect to server
			conn, err := net.Dial("tcp", tcpAddr)
			if err != nil {
				t.Fatalf("Failed to connect to server: %v", err)
			}
			defer conn.Close()

			// Send message
			data, err := tt.message.Serialize()
			if err != nil {
				t.Fatalf("Failed to serialize message: %v", err)
			}

			if _, err := conn.Write(data); err != nil {
				t.Fatalf("Failed to send message: %v", err)
			}

			// Read response
			header := make([]byte, 6)
			if _, err := conn.Read(header); err != nil {
				if !tt.wantErr {
					t.Errorf("Failed to read response header: %v", err)
				}
				return
			}

			response, err := protocol.Deserialize(header)
			if err != nil {
				t.Fatalf("Failed to deserialize response: %v", err)
			}

			if response.PayloadSize > 0 {
				payload := make([]byte, response.PayloadSize)
				if _, err := conn.Read(payload); err != nil {
					t.Fatalf("Failed to read response payload: %v", err)
				}
				response.Payload = payload
			}

			// Verify response
			switch tt.message.Type {
			case protocol.Hello:
				if response.Type != protocol.Hello {
					t.Errorf("Expected Hello response, got %v", response.Type)
				}
			case protocol.Register:
				if response.Type != protocol.Response {
					t.Errorf("Expected Response type, got %v", response.Type)
				}
			case protocol.MCPBridgeAdvertise:
				if response.Type != protocol.Response {
					t.Errorf("Expected Response type, got %v", response.Type)
				}

				// Verify bridge notification
				select {
				case bridge := <-bridgeReceived:
					if bridge.ID != "test-bridge" {
						t.Errorf("Expected bridge ID test-bridge, got %s", bridge.ID)
					}
				case <-time.After(time.Second):
					t.Error("Timeout waiting for bridge notification")
				}
			}
		})
	}
}

func TestUDPServer(t *testing.T) {
	// Create handler
	handler := protocol.NewHandler(nil, nil)

	// Start server
	server := NewServer("127.0.0.1:0", "127.0.0.1:0", handler)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get actual UDP address
	udpAddr := server.udpConn.LocalAddr().String()

	// Create UDP connection
	conn, err := net.Dial("udp", udpAddr)
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// Test query message
	query := struct {
		CapabilityType string `json:"capability_type"`
		MCPEnabled     bool   `json:"mcp_enabled"`
	}{
		CapabilityType: "DISCOVER",
		MCPEnabled:     true,
	}

	msg := &protocol.Message{
		Version:   protocol.V1,
		Type:     protocol.Query,
		Payload:  mustMarshal(t, query),
		Timestamp: time.Now(),
	}

	data, err := msg.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	// Send query
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Failed to send UDP message: %v", err)
	}

	// Read response
	buffer := make([]byte, 65535)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read UDP response: %v", err)
	}

	response, err := protocol.Deserialize(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to deserialize UDP response: %v", err)
	}

	if response.Type != protocol.Response {
		t.Errorf("Expected Response type, got %v", response.Type)
	}
}

func TestGracefulShutdown(t *testing.T) {
	handler := protocol.NewHandler(nil, nil)
	server := NewServer("127.0.0.1:0", "127.0.0.1:0", handler)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Create test connection
	tcpAddr := server.tcpListener.Addr().String()
	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}

	// Start shutdown
	shutdownDone := make(chan struct{})
	go func() {
		if err := server.Stop(); err != nil {
			t.Errorf("Server.Stop() error = %v", err)
		}
		close(shutdownDone)
	}()

	// Verify shutdown completes
	select {
	case <-shutdownDone:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Server shutdown timed out")
	}

	// Verify connection is closed
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Error("Expected connection to be closed")
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	return data
}
