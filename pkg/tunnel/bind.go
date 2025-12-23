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
	"time"
	"sync"

	"github.com/MDSLab/wstun/pkg/logger"
	"github.com/gorilla/websocket"
)

// BindSockets creates a bidirectional tunnel between a WebSocket connection and a TCP connection
func BindSockets(wsConn *websocket.Conn, tcpConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Buffer pool to reduce allocations
	var bufPool = sync.Pool{
		New: func() interface{} { return make([]byte, 32*1024) },
	}

	// Try to tune TCP socket options when possible
	if tc, ok := tcpConn.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(30 * time.Second)
		// Increase kernel buffers; best-effort (ignore errors)
		_ = tc.SetReadBuffer(64 * 1024)
		_ = tc.SetWriteBuffer(64 * 1024)
	}

	// Configure WebSocket connection for liveness detection
	wsConn.SetReadLimit(1024 * 1024) // 1MB max message size
	wsConn.SetPongHandler(func(appData string) error {
		// no-op; pong will extend read deadline if used
		return nil
	})

	// Periodic pinger to detect dead peers
	pingStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				wsConn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(10*time.Second))
			case <-pingStop:
				return
			}
		}
	}()

	// WebSocket -> TCP
	go func() {
		defer wg.Done()
		defer tcpConn.Close()

		for {
			messageType, r, err := wsConn.NextReader()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Debug("WS NextReader error: %v", err)
				}
				return
			}

			if messageType == websocket.TextMessage {
				logger.Warn("Received unexpected text message on WebSocket")
				if rc, ok := r.(io.ReadCloser); ok {
					rc.Close()
				}
				continue
			}

			buf := bufPool.Get().([]byte)
			_, err = io.CopyBuffer(tcpConn, r, buf)
			if rc, ok := r.(io.ReadCloser); ok {
				rc.Close()
			}
			bufPool.Put(buf)

			if err != nil {
				if err != io.EOF {
					logger.Debug("TCP write error: %v", err)
				}
				return
			}
		}
	}()

	// TCP -> WebSocket
	go func() {
		defer wg.Done()
		defer wsConn.Close()

		buf := bufPool.Get().([]byte)
		defer bufPool.Put(buf)

		for {
			n, err := tcpConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					logger.Debug("TCP read error: %v", err)
				}
				return
			}

			if n > 0 {
				w, err := wsConn.NextWriter(websocket.BinaryMessage)
				if err != nil {
					logger.Debug("WS NextWriter error: %v", err)
					return
				}

				_, err = w.Write(buf[:n])
				if err1 := w.Close(); err == nil {
					err = err1
				}

				if err != nil {
					logger.Debug("WS write error: %v", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	close(pingStop)
	logger.Info("[SYSTEM] --> Connection closed")
}
