package protocol

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMessageSerialization(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		wantErr bool
	}{
		{
			name: "basic message",
			message: Message{
				Version:   V1,
				Type:     Hello,
				Payload:  []byte("test payload"),
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty payload",
			message: Message{
				Version:   V1,
				Type:     Register,
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.message.Serialize()
			if (err != nil) != tt.wantErr {
				t.Errorf("Message.Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Errorf("Deserialize() error = %v", err)
				return
			}

			if got.Version != tt.message.Version {
				t.Errorf("Version mismatch: got %v, want %v", got.Version, tt.message.Version)
			}

			if got.Type != tt.message.Type {
				t.Errorf("Type mismatch: got %v, want %v", got.Type, tt.message.Type)
			}

			if string(got.Payload) != string(tt.message.Payload) {
				t.Errorf("Payload mismatch: got %v, want %v", string(got.Payload), string(tt.message.Payload))
			}
		})
	}
}

func TestCapabilityRegistration(t *testing.T) {
	handler := NewHandler(nil, nil)

	cap := &Capability{
		ID:          "test-cap",
		Name:        "Test Capability",
		Type:        "DISCOVER",
		Version:     "1.0",
		Interaction: Discover,
		MCPEnabled:  true,
		Metadata: map[string]string{
			"test": "value",
		},
	}

	if err := handler.RegisterCapability(cap); err != nil {
		t.Errorf("RegisterCapability() error = %v", err)
		return
	}

	// Test capability query
	query := struct {
		CapabilityType string `json:"capability_type"`
		MCPEnabled     bool   `json:"mcp_enabled"`
	}{
		CapabilityType: "DISCOVER",
		MCPEnabled:     true,
	}

	payload, _ := json.Marshal(query)
	msg := &Message{
		Version:   V1,
		Type:     Query,
		Payload:  payload,
		Timestamp: time.Now(),
	}

	response, err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("HandleMessage() error = %v", err)
		return
	}

	if response.Type != Response {
		t.Errorf("Expected response type %v, got %v", Response, response.Type)
	}

	var matches []*Capability
	if err := json.Unmarshal(response.Payload, &matches); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
		return
	}

	if len(matches) != 1 {
		t.Errorf("Expected 1 capability match, got %d", len(matches))
		return
	}

	if matches[0].ID != cap.ID {
		t.Errorf("Expected capability ID %s, got %s", cap.ID, matches[0].ID)
	}
}

func TestMCPBridgeIntegration(t *testing.T) {
	bridgeReceived := make(chan *MCPBridge, 1)
	handler := NewHandler(nil, func(bridge *MCPBridge) error {
		bridgeReceived <- bridge
		return nil
	})

	bridge := &MCPBridge{
		ID:       "test-bridge",
		Endpoint: "mcp://test.endpoint/v1",
		Protocol: "MCP/1.0",
		DataTypes: []string{
			"test_data",
		},
		Metadata: map[string]string{
			"provider": "TestProvider",
		},
		LastUpdated: time.Now(),
	}

	// Register bridge
	bridgeData, _ := json.Marshal(bridge)
	msg := &Message{
		Version:   V1,
		Type:     MCPBridgeAdvertise,
		Payload:  bridgeData,
		Timestamp: time.Now(),
	}

	response, err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("HandleMessage() error = %v", err)
		return
	}

	if response.Type != Response {
		t.Errorf("Expected response type %v, got %v", Response, response.Type)
	}

	// Verify bridge notification
	select {
	case receivedBridge := <-bridgeReceived:
		if receivedBridge.ID != bridge.ID {
			t.Errorf("Expected bridge ID %s, got %s", bridge.ID, receivedBridge.ID)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for bridge notification")
	}

	// Test bridge request
	request := struct {
		BridgeID string `json:"bridge_id"`
		DataType string `json:"data_type"`
	}{
		BridgeID: bridge.ID,
		DataType: "test_data",
	}

	requestData, _ := json.Marshal(request)
	msg = &Message{
		Version:   V1,
		Type:     MCPBridgeRequest,
		Payload:  requestData,
		Timestamp: time.Now(),
	}

	response, err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("HandleMessage() error = %v", err)
		return
	}

	if response.Type != MCPBridgeResponse {
		t.Errorf("Expected response type %v, got %v", MCPBridgeResponse, response.Type)
	}

	var responseBridge MCPBridge
	if err := json.Unmarshal(response.Payload, &responseBridge); err != nil {
		t.Errorf("Failed to unmarshal bridge response: %v", err)
		return
	}

	if responseBridge.ID != bridge.ID {
		t.Errorf("Expected bridge ID %s, got %s", bridge.ID, responseBridge.ID)
	}
}
