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
	"net/url"
	"strings"

	"github.com/MDSLab/wstun/pkg/logger"
	"github.com/gorilla/websocket"
)

// Client represents a WebSocket tunnel client
type Client struct {
	localPort  string
	wsHostURL  string
	remoteAddr string
}

// NewClient creates a new tunnel client
func NewClient() *Client {
	return &Client{}
}

// Start starts the tunnel client
func (c *Client) Start(localPort, wsHostURL, remoteAddr string) error {
	c.localPort = localPort
	c.wsHostURL = wsHostURL
	c.remoteAddr = remoteAddr

	logger.Info("[SYSTEM] - WS Tunnel Client starting...")

	// Create TCP server to listen for local connections
	listener, err := net.Listen("tcp", ":"+localPort)
	if err != nil {
		return fmt.Errorf("failed to create TCP listener: %v", err)
	}
	defer listener.Close()

	logger.Info("[SYSTEM] --> WS tunnel established. Waiting for incoming connections...")

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			logger.Error("Failed to accept TCP connection: %v", err)
			continue
		}

		go c.handleConnection(tcpConn)
	}
}

func (c *Client) handleConnection(tcpConn net.Conn) {
	logger.Info("[SYSTEM] - New connection...")

	// Build WebSocket URL
	wsURL := c.wsHostURL
	if c.remoteAddr != "" {
		wsURL = fmt.Sprintf("%s/?dst=%s", c.wsHostURL, c.remoteAddr)
	}

	// Check if we need TLS
	dialer := websocket.DefaultDialer
	if strings.HasPrefix(wsURL, "wss://") {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // For self-signed certificates
		}
	}

	// Parse URL to add protocol if needed
	parsedURL, err := url.Parse(wsURL)
	if err != nil {
		logger.Error("Invalid WebSocket URL: %v", err)
		tcpConn.Close()
		return
	}

	// Connect to WebSocket server
	header := make(map[string][]string)
	header["Sec-WebSocket-Protocol"] = []string{"tunnel-protocol"}

	wsConn, _, err := dialer.Dial(parsedURL.String(), header)
	if err != nil {
		logger.Error("[SYSTEM] --> WS connect error: %v", err)
		tcpConn.Close()
		return
	}

	logger.Info("[SYSTEM] --> WS connected.")

	// Bind the sockets
	BindSockets(wsConn, tcpConn)
}
