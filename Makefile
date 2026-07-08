BIN_DIR := bin

.PHONY: build build-client build-authority build-transaction fmt test clean

build: build-client build-authority build-transaction

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build-client: $(BIN_DIR)
	cd client && go build -o ../$(BIN_DIR)/client .

build-authority: $(BIN_DIR)
	cd authority-server && go build -o ../$(BIN_DIR)/authority-server .

build-transaction: $(BIN_DIR)
	cd transaction-server && go build -o ../$(BIN_DIR)/transaction-server .

fmt:
	cd client && gofmt -w .
	cd authority-server && gofmt -w .
	cd transaction-server && gofmt -w .

test:
	cd client && go test ./...
	cd authority-server && go test ./...
	cd transaction-server && go test ./...

clean:
	rm -rf $(BIN_DIR)
