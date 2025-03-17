package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/heathweaver/arn-protocol/pkg/network"
	"github.com/heathweaver/arn-protocol/pkg/protocol"
)

var (
	tcpAddr = flag.String("tcp", ":7777", "TCP address to listen on")
	udpAddr = flag.String("udp", ":7778", "UDP address to listen on")
)

// MCPBridgeManager handles MCP data source integration
type MCPBridgeManager struct {
	bridges sync.Map // thread-safe map of MCP bridges
}

func newMCPBridgeManager() *MCPBridgeManager {
	return &MCPBridgeManager{}
}

func (m *MCPBridgeManager) handleMCPBridge(bridge *protocol.MCPBridge) error {
	log.Printf("Registering MCP bridge: %s (%s)", bridge.ID, bridge.Endpoint)
	m.bridges.Store(bridge.ID, bridge)
	return nil
}

func main() {
	flag.Parse()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize MCP bridge manager
	mcpManager := newMCPBridgeManager()

	// Initialize protocol handler with MCP bridge support
	handler := protocol.NewHandler(
		func(msg *protocol.Message) error {
			// Global message handler
			log.Printf("Received message type: %v", msg.Type)
			return nil
		},
		mcpManager.handleMCPBridge,
	)

	// Create and start server
	server := network.NewServer(*tcpAddr, *udpAddr, handler)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop server
	if err := server.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	// Wait for context to be done
	<-shutdownCtx.Done()
	if err := shutdownCtx.Err(); err != context.DeadlineExceeded {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// Example capability registration
func registerExampleCapabilities(handler *protocol.Handler) {
	capabilities := []*protocol.Capability{
		{
			ID:          "ai-discovery",
			Name:        "AI Service Discovery",
			Type:        "DISCOVER",
			Version:     "1.0",
			Interaction: protocol.Discover,
			Metadata: map[string]string{
				"protocol": "ARN/1.0",
			},
		},
		{
			ID:          "data-bridge",
			Name:        "Data Source Bridge",
			Type:        "DELEGATE",
			Version:     "1.0",
			Interaction: protocol.Delegate,
			MCPEnabled:  true,
			Metadata: map[string]string{
				"protocols": "MCP/1.0",
				"data_types": "structured,unstructured,stream",
			},
		},
	}

	for _, cap := range capabilities {
		if err := handler.RegisterCapability(cap); err != nil {
			log.Printf("Failed to register capability %s: %v", cap.ID, err)
		}
	}
}
