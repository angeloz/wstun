// Copyright (C) 2025 wstun-go contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tunnel

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/MDSLab/wstun/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// AllowListEntry represents an entry in the allowlist
type AllowListEntry struct {
	Client string `json:"client"`
	Port   string `json:"port"`
}

// ServerReverseConfig holds the configuration for the reverse tunnel server
type ServerReverseConfig struct {
	SSL       bool
	CertFile  string
	KeyFile   string
	AllowFile string
}

// ServerReverse represents a WebSocket reverse tunnel server
type ServerReverse struct {
	config      ServerReverseConfig
	upgrader    websocket.Upgrader
	allowList   []AllowListEntry
	connections map[string]chan *websocket.Conn
	mu          sync.RWMutex
}

// NewServerReverse creates a new reverse tunnel server
func NewServerReverse(config ServerReverseConfig) *ServerReverse {
	server := &ServerReverse{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
			Subprotocols: []string{"tunnel-protocol"},
		},
		connections: make(map[string]chan *websocket.Conn),
	}

	// Load allowlist if specified
	if config.AllowFile != "" {
		if err := server.loadAllowList(); err != nil {
			logger.Error("Failed to load allowlist: %v", err)
		}
	}

	return server
}

func (s *ServerReverse) loadAllowList() error {
	data, err := ioutil.ReadFile(s.config.AllowFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.allowList)
}

func (s *ServerReverse) isAllowed(clientUUID, port string) bool {
	if s.config.AllowFile == "" {
		return true // No allowlist configured, allow all
	}

	for _, entry := range s.allowList {
		if entry.Client == clientUUID && entry.Port == port {
			return true
		}
	}
	return false
}

// Start starts the reverse tunnel server
func (s *ServerReverse) Start(port string) error {
	logger.Info("[SYSTEM] - WS Reverse Tunnel Server starting...")

	if s.config.AllowFile != "" {
		logger.Info("  AllowList: %s", s.config.AllowFile)
	}

	// Set up HTTP handler
	http.HandleFunc("/", s.handleHTTP)

	addr := ":" + port

	if s.config.SSL {
		logger.Info("[SYSTEM] - WS Reverse Tunnel Server over HTTPS.")
		logger.Info("[SYSTEM] - WS Reverse Tunnel Server starting on: wss://localhost:%s", port)

		// Load TLS certificates
		cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load certificates: %v", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		server := &http.Server{
			Addr:      addr,
			TLSConfig: tlsConfig,
		}

		logger.Info("[SYSTEM] - WS Reverse Tunnel Server is listening...")
		return server.ListenAndServeTLS("", "")
	}

	logger.Info("[SYSTEM] - WS Reverse Tunnel Server over HTTP.")
	logger.Info("[SYSTEM] - WS Reverse Tunnel Server starting on: ws://localhost:%s", port)
	logger.Info("[SYSTEM] - WS Reverse Tunnel Server is listening...")

	return http.ListenAndServe(addr, nil)
}

func (s *ServerReverse) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket upgrade request
	if websocket.IsWebSocketUpgrade(r) {
		s.handleWebSocket(w, r)
		return
	}

	// Regular HTTP request
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "<!DOCTYPE html><html><head><title>WSTUN</title></head><body>iotronic-wstun is running!</body></html>")
}

func (s *ServerReverse) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	queryParams, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		logger.Error("Failed to parse query: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	dst := queryParams.Get("dst")
	id := queryParams.Get("id")

	if dst != "" {
		// This is a control connection
		s.handleControlConnection(w, r, queryParams)
	} else if id != "" {
		// This is a data connection
		s.handleDataConnection(w, r, id)
	} else {
		logger.Error("Invalid WebSocket request: missing dst or id parameter")
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
}

func (s *ServerReverse) handleControlConnection(w http.ResponseWriter, r *http.Request, queryParams url.Values) {
	wsConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed: %v", err)
		return
	}

	dst := queryParams.Get("dst")
	clientUUID := queryParams.Get("uuid")

	parts := strings.Split(dst, ":")
	if len(parts) != 2 {
		logger.Error("Invalid destination format: %s", dst)
		wsConn.Close()
		return
	}

	portTCP := parts[1]

	srcAddr := r.RemoteAddr
	if clientUUID != "" {
		logger.Info("[SYSTEM] WebSocket creation towards %s on port %s from client %s", srcAddr, portTCP, clientUUID)
	} else {
		logger.Info("[SYSTEM] WebSocket creation towards %s on port %s", srcAddr, portTCP)
	}

	// Check allowlist
	if s.config.AllowFile != "" {
		if clientUUID == "" {
			logger.Warn("Client UUID not specified, connection not authorized")
			wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Unauthorized"))
			wsConn.Close()
			return
		}

		if !s.isAllowed(clientUUID, portTCP) {
			logger.Warn("Port %s not allowed for client %s", portTCP, clientUUID)
			wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Unauthorized"))
			wsConn.Close()
			return
		}

		logger.Info("[SYSTEM] --> Client %s authorized for port %s", clientUUID, portTCP)
	}

	// Create TCP listener
	listener, err := net.Listen("tcp", ":"+portTCP)
	if err != nil {
		logger.Error("[SYSTEM] - Error - Port %s already used or cannot bind: %v", portTCP, err)
		wsConn.Close()
		return
	}

	logger.Info("[SYSTEM] --> TCP server is listening on port %s", portTCP)
	logger.Info("[SYSTEM] --> WS connection created")

	// Handle WebSocket close
	go func() {
		for {
			_, _, err := wsConn.ReadMessage()
			if err != nil {
				logger.Info("[SYSTEM] - WebSocket Control Peer %s disconnected", wsConn.RemoteAddr())
				logger.Info("[SYSTEM] --> Close TCP server on port %s", portTCP)
				listener.Close()
				return
			}
		}
	}()

	// Accept TCP connections
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			// Listener closed
			return
		}

		go s.handleTCPConnection(wsConn, tcpConn)
	}
}

func (s *ServerReverse) handleTCPConnection(controlWS *websocket.Conn, tcpConn net.Conn) {
	// Generate unique ID for this connection
	connID := uuid.New().String()

	// Create channel to receive data WebSocket
	dataChan := make(chan *websocket.Conn, 1)
	s.mu.Lock()
	s.connections[connID] = dataChan
	s.mu.Unlock()

	// Send new connection message to client
	msg := fmt.Sprintf("NC:%s", connID)
	err := controlWS.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		logger.Error("Failed to send NC message: %v", err)
		tcpConn.Close()
		s.mu.Lock()
		delete(s.connections, connID)
		s.mu.Unlock()
		return
	}

	// Wait for data WebSocket connection (with timeout)
	dataWS := <-dataChan

	s.mu.Lock()
	delete(s.connections, connID)
	s.mu.Unlock()

	if dataWS == nil {
		logger.Error("No data WebSocket received for connection %s", connID)
		tcpConn.Close()
		return
	}

	// Bind the connections
	BindSockets(dataWS, tcpConn)
}

func (s *ServerReverse) handleDataConnection(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.RLock()
	dataChan, exists := s.connections[id]
	s.mu.RUnlock()

	if !exists {
		logger.Error("No pending connection for ID %s", id)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	wsConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed: %v", err)
		return
	}

	logger.Info("[SYSTEM] --> WebSocket Request for Data (ID: %s)", id)

	// Send the WebSocket to the waiting goroutine
	dataChan <- wsConn
}
