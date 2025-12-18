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
	// Allow `-r`/`--reverse` to be used either alone (as boolean, e.g. server reverse)
	// or with a value (port:host:port) like the Node.js version. To support the
	// "-r" without a value we pre-scan `os.Args` and insert an explicit empty
	// string as the value when `-r` is present and the next token is another
	// flag or missing. This mirrors the Node.js `optimist` behaviour.
	reverseProvided := false
	// Build newArgs from os.Args, inserting an empty value for -r when needed
	if len(os.Args) > 1 {
		newArgs := make([]string, 0, len(os.Args))
		newArgs = append(newArgs, os.Args[0])
		for i := 1; i < len(os.Args); i++ {
			a := os.Args[i]
			if a == "-r" || a == "--reverse" {
				reverseProvided = true
				newArgs = append(newArgs, a)
				// If next token is missing or is another flag, insert empty value
				if i+1 >= len(os.Args) || strings.HasPrefix(os.Args[i+1], "-") {
					newArgs = append(newArgs, "")
				}
				continue
			}
			newArgs = append(newArgs, a)
		}
		os.Args = newArgs
	}

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
	dryRun := flag.Bool("dry-run", false, "Print parsed arguments and exit (for testing)")
	help := flag.Bool("h", false, "Show help message")

	flag.Parse()

	// Show help
	if *help {
		fmt.Println(usage)
		os.Exit(0)
	}

	// (dry-run handled after positional args are extracted below)

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

	// Dry-run: print parsed arguments and exit for functional testing
	if *dryRun {
		fmt.Printf("PARSED server=%q tunnel=%q reverse=%q reverseProvided=%v allow=%q uuid=%q ssl=%q key=%q cert=%q wsHost=%q\n",
			*serverPort, *tunnelSpec, *reverseSpec, reverseProvided, *allowFile, *clientUUID, *sslFlag, *keyFile, *certFile, wsHostURL)
		os.Exit(0)
	}

	// Determine mode and start appropriate component. Match Node.js behavior:
	// 1) If `-s` provided and `-r` not provided => forward server
	// 2) Else if `-t` provided => forward client
	// 3) Else if `-r` provided => if `-s` provided => reverse server else reverse client
	if *serverPort != "" && !reverseProvided {
		// Forward tunnel server
		runForwardServer(*serverPort, *tunnelSpec, useSSL, *keyFile, *certFile)
	} else if *tunnelSpec != "" {
		// Forward tunnel client
		runForwardClient(*tunnelSpec, wsHostURL)
	} else if reverseProvided {
		if *serverPort != "" {
			// Reverse tunnel server (user passed -r without a value)
			runReverseServer(*serverPort, useSSL, *keyFile, *certFile, *allowFile)
		} else {
			// Reverse tunnel client: require a non-empty reverseSpec
			if *reverseSpec == "" {
				logger.Fatal("Reverse tunnel specification missing (expected portTunnel:host:port)")
			}
			runReverseClient(*reverseSpec, wsHostURL, *clientUUID)
		}
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
