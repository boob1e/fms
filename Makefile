
.PHONY: build run clean dev test

# Binary name
BINARY_NAME=folo

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) main.go
	@echo "Build complete: bin/$(BINARY_NAME)"

# Build and run the application
run: build
	@echo "Starting $(BINARY_NAME)..."
	@./bin/$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean
	@echo "Clean complete"

# Run with go run (no build)
dev:
	@echo "Running in development mode..."
	@go run main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"
