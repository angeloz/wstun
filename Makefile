# Makefile for wstun-go
# Cross-compilation for embedded systems

VERSION := 1.1.0
BINARY_NAME := wstun
BUILD_DIR := build
CMD_DIR := cmd/wstun

# Build flags for smaller binaries
LDFLAGS := -ldflags="-s -w"

.PHONY: all clean build-all help

all: build

help:
	@echo "wstun-go Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make build              - Build for current platform"
	@echo "  make build-all          - Build for all embedded platforms"
	@echo "  make linux-amd64        - Build for Linux AMD64"
	@echo "  make linux-arm          - Build for Linux ARM (32-bit)"
	@echo "  make linux-arm64        - Build for Linux ARM64"
	@echo "  make linux-mips         - Build for Linux MIPS (big-endian)"
	@echo "  make linux-mipsle       - Build for Linux MIPS (little-endian)"
	@echo "  make openwrt-arm        - Build for OpenWRT ARM"
	@echo "  make openwrt-mips       - Build for OpenWRT MIPS"
	@echo "  make clean              - Clean build artifacts"
	@echo ""

build:
	@echo "Building for current platform..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Binary created: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: linux-amd64 linux-386 linux-arm linux-arm64 linux-mips linux-mipsle
	@echo "All binaries built successfully!"
	@ls -lh $(BUILD_DIR)

linux-amd64:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)

linux-386:
	@echo "Building for Linux 386..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=386 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-386 ./$(CMD_DIR)

linux-arm:
	@echo "Building for Linux ARM (32-bit)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm ./$(CMD_DIR)

linux-arm64:
	@echo "Building for Linux ARM64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

linux-mips:
	@echo "Building for Linux MIPS (big-endian)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=mips GOMIPS=softfloat go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-mips ./$(CMD_DIR)

linux-mipsle:
	@echo "Building for Linux MIPS (little-endian)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-mipsle ./$(CMD_DIR)

# OpenWRT specific builds
openwrt-arm: linux-arm
	@echo "OpenWRT ARM binary: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm"

openwrt-mips: linux-mipsle
	@echo "OpenWRT MIPS binary: $(BUILD_DIR)/$(BINARY_NAME)-linux-mipsle"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run tests
test:
	go test -v ./...

# Install dependencies
deps:
	go mod download
	go mod tidy

# Show binary sizes
sizes:
	@echo "Binary sizes:"
	@ls -lh $(BUILD_DIR) | grep wstun
