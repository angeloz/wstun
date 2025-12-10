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

// ClientReverse represents a reverse tunnel client
type ClientReverse struct {
	portTunnel string
	wsHostURL  string
	remoteAddr string
	uuid       string
	controlWS  *websocket.Conn
}

// NewClientReverse creates a new reverse tunnel client
func NewClientReverse() *ClientReverse {
	return &ClientReverse{}
}

// Start starts the reverse tunnel client
func (c *ClientReverse) Start(portTunnel, wsHostURL, remoteAddr, clientUUID string) error {
	c.portTunnel = portTunnel
	c.wsHostURL = wsHostURL
	c.remoteAddr = remoteAddr
	c.uuid = clientUUID

	// Parse remote address
	parts := strings.Split(remoteAddr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid remote address format: %s", remoteAddr)
	}
	remoteHost := parts[0]
	remotePort := parts[1]

	// Parse WebSocket host URL
	parsedURL, err := url.Parse(wsHostURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %v", err)
	}

	// Build control connection URL
	controlURL := fmt.Sprintf("%s/?dst=%s:%s", wsHostURL, parsedURL.Hostname(), portTunnel)
	if clientUUID != "" {
		controlURL += "&uuid=" + clientUUID
	}

	logger.Info("[SYSTEM] -------------------- Connecting to %s", wsHostURL)
	logger.Info("[SYSTEM] --------------------> exposing %s on port %s", remoteAddr, portTunnel)
	if clientUUID != "" {
		logger.Info("[SYSTEM] --> My UUID is %s", clientUUID)
	}

	// Set up WebSocket dialer
	dialer := websocket.DefaultDialer
	if strings.HasPrefix(wsHostURL, "wss://") {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // For self-signed certificates
		}
	}

	// Connect to control WebSocket
	header := make(map[string][]string)
	header["Sec-WebSocket-Protocol"] = []string{"tunnel-protocol"}

	controlWS, _, err := dialer.Dial(controlURL, header)
	if err != nil {
		return fmt.Errorf("failed to connect to control WebSocket: %v", err)
	}

	c.controlWS = controlWS
	logger.Info("[SYSTEM] --> TCP connection established!")

	// Listen for messages from control WebSocket
	for {
		messageType, message, err := controlWS.ReadMessage()
		if err != nil {
			logger.Error("Control WebSocket read error: %v", err)
			return err
		}

		if messageType == websocket.TextMessage {
			msg := string(message)
			parts := strings.Split(msg, ":")

			if len(parts) == 2 && parts[0] == "NC" {
				// New connection request
				connID := parts[1]
				logger.Debug("New connection request with ID: %s", connID)

				go c.handleNewConnection(connID, remoteHost, remotePort)
			}
		}
	}
}

func (c *ClientReverse) handleNewConnection(connID, remoteHost, remotePort string) {
	// Connect to data WebSocket
	dataURL := fmt.Sprintf("%s/?id=%s", c.wsHostURL, connID)

	dialer := websocket.DefaultDialer
	if strings.HasPrefix(c.wsHostURL, "wss://") {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	header := make(map[string][]string)
	header["Sec-WebSocket-Protocol"] = []string{"tunnel-protocol"}

	dataWS, _, err := dialer.Dial(dataURL, header)
	if err != nil {
		logger.Error("Failed to connect data WebSocket: %v", err)
		return
	}

	logger.Info("[SYSTEM] --> Start TCP connection on client to %s:%s", remoteHost, remotePort)

	// Connect to local service
	tcpConn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", remoteHost, remotePort))
	if err != nil {
		logger.Error("[SYSTEM] --> TCP connection error: %v", err)
		dataWS.Close()
		return
	}

	// Bind the connections
	BindSockets(dataWS, tcpConn)
}
