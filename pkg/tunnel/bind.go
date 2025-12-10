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
	"io"
	"net"
	"sync"

	"github.com/MDSLab/wstun/pkg/logger"
	"github.com/gorilla/websocket"
)

// BindSockets creates a bidirectional tunnel between a WebSocket connection and a TCP connection
func BindSockets(wsConn *websocket.Conn, tcpConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// WebSocket -> TCP
	go func() {
		defer wg.Done()
		defer tcpConn.Close()

		for {
			messageType, message, err := wsConn.ReadMessage()
			if err != nil {
				if err != io.EOF && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Debug("WS read error: %v", err)
				}
				return
			}

			if messageType == websocket.TextMessage {
				logger.Warn("Received unexpected text message on WebSocket")
				continue
			}

			if messageType == websocket.BinaryMessage {
				_, err := tcpConn.Write(message)
				if err != nil {
					logger.Debug("TCP write error: %v", err)
					return
				}
			}
		}
	}()

	// TCP -> WebSocket
	go func() {
		defer wg.Done()
		defer wsConn.Close()

		buffer := make([]byte, 32*1024) // 32KB buffer
		for {
			n, err := tcpConn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.Debug("TCP read error: %v", err)
				}
				return
			}

			if n > 0 {
				err = wsConn.WriteMessage(websocket.BinaryMessage, buffer[:n])
				if err != nil {
					logger.Debug("WS write error: %v", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	logger.Info("[SYSTEM] --> Connection closed")
}
