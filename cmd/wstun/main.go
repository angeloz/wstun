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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/MDSLab/wstun/pkg/logger"
	"github.com/MDSLab/wstun/pkg/tunnel"
)

const usage = `wstun - WebSocket Tunneling Tool (Go Edition)

Tunnels and reverse tunnels over WebSocket.

Usage: https://github.com/MDSLab/wstun

Options:
  -s, --server <port>     Run as server, specify listening port
  -t, --tunnel <spec>     Run as tunnel client, specify localport:host:port
  -r, --reverse <spec>    Run in reverse tunneling mode
  -a, --allow <file>      [server + reverse] Accept only authorized clients (allowlist file)
  -u, --uuid <uuid>       [client + reverse] Specify the UUID of the client
  --ssl <true|false>      Enable or disable HTTPS/WSS communication
  --key <file>            [with --ssl=true] Path to private key certificate
  --cert <file>           [with --ssl=true] Path to public key certificate
  --log <file>            Path to log file (default: stdout)
  --debug                 Enable debug logging
  -h, --help              Show this help message

Examples:
  # Forward tunnel server
  wstun -s 8080

  # Forward tunnel client
  wstun -t 33:remotehost:33 ws://serverhost:8080

  # Reverse tunnel server
  wstun -r -s 8080

  # Reverse tunnel client
  wstun -r 33:localhost:22 -u client-uuid ws://serverhost:8080

  # With SSL/TLS
  wstun -s 8443 --ssl true --key server.key --cert server.crt
`

func main() {
	// Define flags
	serverPort := flag.String("s", "", "Run as server, specify listening port")
	tunnelSpec := flag.String("t", "", "Run as tunnel client (localport:host:port)")
	reverseSpec := flag.String("r", "", "Run in reverse tunneling mode")
	allowFile := flag.String("a", "", "Allowlist file for authorized clients")
	clientUUID := flag.String("u", "", "Client UUID for reverse tunnel")
	sslFlag := flag.String("ssl", "", "Enable SSL/TLS (true/false)")
	keyFile := flag.String("key", "", "Path to private key certificate")
	certFile := flag.String("cert", "", "Path to public key certificate")
	logFile := flag.String("log", "", "Path to log file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	help := flag.Bool("h", false, "Show help message")

	flag.Parse()

	// Show help
	if *help {
		fmt.Println(usage)
		os.Exit(0)
	}

	// Set up logging
	if *debug {
		logger.SetLevel(logger.DEBUG)
	}

	if *logFile != "" {
		if err := logger.SetLogFile(*logFile); err != nil {
			logger.Error("Failed to open log file: %v", err)
			os.Exit(1)
		}
		defer logger.Close()
	}

	// Parse SSL flag
	useSSL := false
	if *sslFlag == "true" {
		useSSL = true
		if *keyFile == "" || *certFile == "" {
			logger.Error("SSL enabled but key or cert file not specified")
			fmt.Println(usage)
			os.Exit(1)
		}
	}

	// Get WebSocket host URL from remaining args
	args := flag.Args()
	var wsHostURL string
	if len(args) > 0 {
		wsHostURL = args[len(args)-1]
	}

	// Determine mode and start appropriate component
	if *serverPort != "" && *reverseSpec == "" {
		// Forward tunnel server
		runForwardServer(*serverPort, *tunnelSpec, useSSL, *keyFile, *certFile)
	} else if *tunnelSpec != "" && *reverseSpec == "" {
		// Forward tunnel client
		runForwardClient(*tunnelSpec, wsHostURL)
	} else if *reverseSpec != "" && *serverPort != "" {
		// Reverse tunnel server
		runReverseServer(*serverPort, useSSL, *keyFile, *certFile, *allowFile)
	} else if *reverseSpec != "" && *serverPort == "" {
		// Reverse tunnel client
		runReverseClient(*reverseSpec, wsHostURL, *clientUUID)
	} else {
		fmt.Println(usage)
		os.Exit(1)
	}
}

func runForwardServer(port, tunnelSpec string, ssl bool, keyFile, certFile string) {
	config := tunnel.ServerConfig{
		SSL:      ssl,
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	// Parse tunnel spec if provided
	if tunnelSpec != "" {
		parts := strings.Split(tunnelSpec, ":")
		if len(parts) == 2 {
			config.DstHost = parts[0]
			config.DstPort = parts[1]
		}
	}

	server := tunnel.NewServer(config)
	if err := server.Start(port); err != nil {
		logger.Fatal("Server error: %v", err)
	}
}

func runForwardClient(tunnelSpec, wsHostURL string) {
	if wsHostURL == "" {
		logger.Fatal("WebSocket host URL not specified")
	}

	parts := strings.Split(tunnelSpec, ":")
	if len(parts) < 1 {
		logger.Fatal("Invalid tunnel specification: %s", tunnelSpec)
	}

	localPort := parts[0]
	remoteAddr := ""

	if len(parts) == 3 {
		remoteAddr = parts[1] + ":" + parts[2]
	}

	client := tunnel.NewClient()
	if err := client.Start(localPort, wsHostURL, remoteAddr); err != nil {
		logger.Fatal("Client error: %v", err)
	}
}

func runReverseServer(port string, ssl bool, keyFile, certFile, allowFile string) {
	config := tunnel.ServerReverseConfig{
		SSL:       ssl,
		CertFile:  certFile,
		KeyFile:   keyFile,
		AllowFile: allowFile,
	}

	server := tunnel.NewServerReverse(config)
	if err := server.Start(port); err != nil {
		logger.Fatal("Reverse server error: %v", err)
	}
}

func runReverseClient(reverseSpec, wsHostURL, clientUUID string) {
	if wsHostURL == "" {
		logger.Fatal("WebSocket host URL not specified")
	}

	parts := strings.Split(reverseSpec, ":")
	if len(parts) != 3 {
		logger.Fatal("Invalid reverse tunnel specification: %s (expected portTunnel:host:port)", reverseSpec)
	}

	portTunnel := parts[0]
	remoteAddr := parts[1] + ":" + parts[2]

	client := tunnel.NewClientReverse()
	if err := client.Start(portTunnel, wsHostURL, remoteAddr, clientUUID); err != nil {
		logger.Fatal("Reverse client error: %v", err)
	}
}
