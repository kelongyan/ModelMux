BINARY  = modelmux.exe
CONFIG  = config.json
BUILDTIME = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOFLAGS = -trimpath -ldflags="-s -w -X github.com/kelongyan/ModelMux/admin.buildTime=$(BUILDTIME)"

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
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o modelmux-windows-amd64.exe .

build-mac:
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o $(BINARY)-darwin-arm64 .
