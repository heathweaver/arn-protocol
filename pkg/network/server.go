package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/heathweaver/arn-protocol/pkg/protocol"
)

// Server represents the ARN network server
type Server struct {
	tcpAddr     string
	udpAddr     string
	handler     *protocol.Handler
	tcpListener net.Listener
	udpConn     *net.UDPConn
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewServer creates a new ARN server
func NewServer(tcpAddr, udpAddr string, handler *protocol.Handler) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		tcpAddr: tcpAddr,
		udpAddr: udpAddr,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins listening for connections
func (s *Server) Start() error {
	// Start TCP listener
	tcpListener, err := net.Listen("tcp", s.tcpAddr)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	s.tcpListener = tcpListener

	// Start UDP listener
	udpAddr, err := net.ResolveUDPAddr("udp", s.udpAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		s.tcpListener.Close()
		return fmt.Errorf("failed to start UDP listener: %w", err)
	}
	s.udpConn = udpConn

	// Start handlers
	s.wg.Add(2)
	go s.handleTCP()
	go s.handleUDP()

	log.Printf("ARN server listening on TCP %s and UDP %s", s.tcpAddr, s.udpAddr)
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.cancel()

	if s.tcpListener != nil {
		if err := s.tcpListener.Close(); err != nil {
			return fmt.Errorf("failed to close TCP listener: %w", err)
		}
	}

	if s.udpConn != nil {
		if err := s.udpConn.Close(); err != nil {
			return fmt.Errorf("failed to close UDP connection: %w", err)
		}
	}

	s.wg.Wait()
	return nil
}

func (s *Server) handleTCP() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.tcpListener.Accept()
			if err != nil {
				if s.ctx.Err() != nil {
					return // Server is shutting down
				}
				log.Printf("Failed to accept TCP connection: %v", err)
				continue
			}

			s.wg.Add(1)
			go s.handleTCPConnection(conn)
		}
	}
}

func (s *Server) handleTCPConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Set reasonable timeouts
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Read message header (version + type + size = 6 bytes)
	header := make([]byte, 6)
	if _, err := conn.Read(header); err != nil {
		log.Printf("Failed to read TCP header: %v", err)
		return
	}

	// Parse message
	msg, err := protocol.Deserialize(header)
	if err != nil {
		log.Printf("Failed to deserialize TCP message: %v", err)
		return
	}

	// Read payload if present
	if msg.PayloadSize > 0 {
		payload := make([]byte, msg.PayloadSize)
		if _, err := conn.Read(payload); err != nil {
			log.Printf("Failed to read TCP payload: %v", err)
			return
		}
		msg.Payload = payload
	}

	// Handle message
	response, err := s.handler.HandleMessage(s.ctx, msg)
	if err != nil {
		log.Printf("Failed to handle TCP message: %v", err)
		return
	}

	// Send response if any
	if response != nil {
		data, err := response.Serialize()
		if err != nil {
			log.Printf("Failed to serialize TCP response: %v", err)
			return
		}

		if _, err := conn.Write(data); err != nil {
			log.Printf("Failed to write TCP response: %v", err)
			return
		}
	}
}

func (s *Server) handleUDP() {
	defer s.wg.Done()

	buffer := make([]byte, 65535) // Maximum UDP packet size

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			n, addr, err := s.udpConn.ReadFromUDP(buffer)
			if err != nil {
				if s.ctx.Err() != nil {
					return // Server is shutting down
				}
				log.Printf("Failed to read UDP packet: %v", err)
				continue
			}

			// Handle packet in a goroutine
			s.wg.Add(1)
			go s.handleUDPPacket(buffer[:n], addr)
		}
	}
}

func (s *Server) handleUDPPacket(data []byte, addr *net.UDPAddr) {
	defer s.wg.Done()

	// Parse message
	msg, err := protocol.Deserialize(data)
	if err != nil {
		log.Printf("Failed to deserialize UDP message: %v", err)
		return
	}

	// Handle message
	response, err := s.handler.HandleMessage(s.ctx, msg)
	if err != nil {
		log.Printf("Failed to handle UDP message: %v", err)
		return
	}

	// Send response if any
	if response != nil {
		data, err := response.Serialize()
		if err != nil {
			log.Printf("Failed to serialize UDP response: %v", err)
			return
		}

		if _, err := s.udpConn.WriteToUDP(data, addr); err != nil {
			log.Printf("Failed to write UDP response: %v", err)
			return
		}
	}
}
