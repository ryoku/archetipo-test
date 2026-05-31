BIN_DIR := bin

.PHONY: build test vet clean

build:
	go build -o $(BIN_DIR)/server ./cmd/server
	go build -o $(BIN_DIR)/kubegate ./cmd/kubegate

test:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)
