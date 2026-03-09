.PHONY: build clean install test

# Build the binary
build:
	go mod tidy
	go build -o ado ./cmd/main.go

# Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -o ado.exe ./cmd/main.go

# Build for Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o ado-linux ./cmd/main.go

# Build for macOS
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o ado-darwin ./cmd/main.go

# Clean build artifacts
clean:
	rm -f ado ado.exe ado-linux ado-darwin

# Install to $GOPATH/bin
install:
	go install ./cmd/main.go

# Run tests
test:
	go test ./...

# Run the CLI
run:
	go run ./cmd/main.go

# Download dependencies
deps:
	go mod download
	go mod tidy
