# Build dbtop binary
build:
    go build -o dbtop .

# Build with version info
build-release version="dev":
    go build -ldflags "-s -w" -o dbtop .

# Run the binary
run *ARGS:
    go run . {{ARGS}}

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
