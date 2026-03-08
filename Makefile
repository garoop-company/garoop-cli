VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST_DIR ?= dist

.PHONY: build build-all snapshot release-check clean

build:
	go build -o garoop-cli ./cmd/garoop-cli
	go build -o garuchan-cli ./cmd/garuchan-cli
	go build -o garooptv-cli ./cmd/garooptv-cli

build-all:
	mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/garoop-cli_darwin_arm64 ./cmd/garoop-cli
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/garuchan-cli_darwin_arm64 ./cmd/garuchan-cli
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/garooptv-cli_darwin_arm64 ./cmd/garooptv-cli
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/garoop-cli_linux_amd64 ./cmd/garoop-cli
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/garuchan-cli_linux_amd64 ./cmd/garuchan-cli
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/garooptv-cli_linux_amd64 ./cmd/garooptv-cli

snapshot:
	goreleaser release --snapshot --clean

release-check:
	go test ./...
	goreleaser check

clean:
	rm -rf $(DIST_DIR) garoop-cli garuchan-cli garooptv-cli
