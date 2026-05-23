BINARY  = claude-key-proxy
CONFIG  = config.json
GOFLAGS = -trimpath -ldflags="-s -w"

.PHONY: build run test clean

build:
	go build $(GOFLAGS) -o $(BINARY) .

run: build
	./$(BINARY) -config $(CONFIG)

test:
	go test ./...

clean:
	rm -f $(BINARY)

# Cross-compile targets
build-linux:
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o $(BINARY)-linux-amd64 .

build-windows:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o $(BINARY)-windows-amd64.exe .

build-mac:
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o $(BINARY)-darwin-arm64 .
