BINARY_NAME=0itotrade-supervisor
BUILD_DIR=dist
VERSION ?= 0.1.0
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildDate=$(BUILD_DATE)'

.PHONY: all clean build build-linux-amd64 build-linux-arm64 package-all

all: build

clean:
	rm -rf $(BUILD_DIR)

build:
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/supervisor

build-linux-amd64:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/supervisor

build-linux-arm64:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/supervisor

package-all: build-linux-amd64 build-linux-arm64
	cd $(BUILD_DIR) && cp $(BINARY_NAME)-linux-amd64 $(BINARY_NAME) && tar -czf $(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	cd $(BUILD_DIR) && cp $(BINARY_NAME)-linux-arm64 $(BINARY_NAME) && tar -czf $(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME) && rm -f $(BINARY_NAME)
	@echo "Pacotes gerados em $(BUILD_DIR)/:"
	@ls -la $(BUILD_DIR)/*.tar.gz
