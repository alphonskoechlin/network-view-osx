.PHONY: build test clean run dev help install-deps buf-gen

BACKEND_DIR := backend
FRONTEND_DIR := frontend
BINARY_NAME := network-view-osx
BACKEND_BINARY := $(BACKEND_DIR)/$(BINARY_NAME)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  install-deps Build backend and frontend"
	@echo "  buf-gen      Generate protobuf code"
	@echo "  build        Build backend and frontend"
	@echo "  backend      Build Go backend only"
	@echo "  frontend     Build Svelte frontend only"
	@echo "  test         Run backend tests"
	@echo "  clean        Clean build artifacts"
	@echo "  run          Build and run backend (frontend on separate terminal)"
	@echo "  dev          Start development mode (backend + frontend)"
	@echo "  help         Show this help message"

install-deps:
	@echo "Installing dependencies..."
	cd $(BACKEND_DIR) && go mod download && go mod tidy
	cd $(FRONTEND_DIR) && npm install
	@echo "✓ Dependencies installed"

buf-gen:
	@echo "Generating protobuf code..."
	cd $(BACKEND_DIR) && buf generate
	@echo "✓ Protobuf code generated"

backend:
	cd $(BACKEND_DIR) && go build -o $(BINARY_NAME)

frontend:
	cd $(FRONTEND_DIR) && npm run build

build: backend frontend
	@echo "✓ Build complete"

test:
	cd $(BACKEND_DIR) && go test -v ./...

clean:
	rm -f $(BACKEND_BINARY)
	cd $(FRONTEND_DIR) && rm -rf dist
	cd $(BACKEND_DIR) && go clean

run: backend
	cd $(BACKEND_DIR) && ./$(BINARY_NAME)

dev: install-deps buf-gen
	@echo "Starting development mode..."
	@echo "Backend will run on http://localhost:8080"
	@echo "Frontend will run on http://localhost:5173"
	@echo ""
	@(cd $(BACKEND_DIR) && go run main.go) & \
	(cd $(FRONTEND_DIR) && npm run dev) & \
	wait
