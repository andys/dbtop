# Build dbtop binary
build:
    go build -o dbtop ./cmd/dbtop

# Build with version info
build-release version="dev":
    go build -ldflags "-s -w" -o dbtop ./cmd/dbtop

# Run the binary
run *ARGS:
    go run ./cmd/dbtop {{ARGS}}

# Clean build artifacts
clean:
    rm -f dbtop

# Run tests
test:
    go test ./...

# Format code
fmt:
    go fmt ./...

# Vet code
vet:
    go vet ./...
