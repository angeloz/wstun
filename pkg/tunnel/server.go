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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/MDSLab/wstun/pkg/logger"
	"github.com/gorilla/websocket"
)

// ServerConfig holds the configuration for the tunnel server
type ServerConfig struct {
	DstHost  string
	DstPort  string
	SSL      bool
	CertFile string
	KeyFile  string
}

// Server represents a WebSocket tunnel server
type Server struct {
	config   ServerConfig
	upgrader websocket.Upgrader
}

// NewServer creates a new tunnel server
func NewServer(config ServerConfig) *Server {
	return &Server{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
			Subprotocols: []string{"tunnel-protocol"},
		},
	}
}

// Start starts the WebSocket tunnel server
func (s *Server) Start(port string) error {
	logger.Info("[SYSTEM] - WS Tunnel Server starting with parameters:")
	logger.Info("  DstHost: %s", s.config.DstHost)
	logger.Info("  DstPort: %s", s.config.DstPort)
	logger.Info("  SSL: %v", s.config.SSL)

	http.HandleFunc("/", s.handleWebSocket)

	addr := ":" + port

	if s.config.SSL {
		logger.Info("[SYSTEM] - WS over HTTPS")
		logger.Info("[SYSTEM] - WS Tunnel Server starting on: wss://localhost:%s", port)

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

		logger.Info("[SYSTEM] - Server is listening on port %s...", port)
		return server.ListenAndServeTLS("", "")
	}

	logger.Info("[SYSTEM] - WS over HTTP")
	logger.Info("[SYSTEM] - WS Tunnel Server starting on: ws://localhost:%s", port)
	logger.Info("[SYSTEM] - Server is listening on port %s...", port)

	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	wsConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed: %v", err)
		return
	}

	// Parse query parameters
	queryParams, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		logger.Error("Failed to parse query: %v", err)
		wsConn.Close()
		return
	}

	// Determine destination
	var host, port string
	if s.config.DstHost != "" && s.config.DstPort != "" {
		host = s.config.DstHost
		port = s.config.DstPort
	} else {
		dst := queryParams.Get("dst")
		if dst == "" {
			logger.Error("No tunnel target specified")
			wsConn.Close()
			return
		}

		parts := strings.Split(dst, ":")
		if len(parts) != 2 {
			logger.Error("Invalid destination format: %s", dst)
			wsConn.Close()
			return
		}
		host = parts[0]
		port = parts[1]
	}

	remoteAddr := fmt.Sprintf("%s:%s", host, port)
	logger.Info("[SYSTEM] - Establishing tunnel to %s", remoteAddr)

	// Connect to destination TCP server
	tcpConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		logger.Error("Tunnel connect error to %s: %v", remoteAddr, err)
		wsConn.Close()
		return
	}

	// Bind the sockets
	BindSockets(wsConn, tcpConn)
}
