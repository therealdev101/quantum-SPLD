# Makefile for building Splendor blockchain with post-quantum cryptography support
# This extends the main Makefile with liboqs integration

# liboqs configuration
LIBOQS_VERSION ?= 0.8.0
LIBOQS_DIR ?= $(PWD)/crypto/liboqs
LIBOQS_BUILD_DIR ?= $(LIBOQS_DIR)/build
LIBOQS_INSTALL_DIR ?= $(LIBOQS_DIR)/install

# Compiler flags for post-quantum support
PQ_CFLAGS = -I$(LIBOQS_INSTALL_DIR)/include
PQ_LDFLAGS = -L$(LIBOQS_INSTALL_DIR)/lib -loqs
PQ_CGO_CFLAGS = CGO_CFLAGS="$(PQ_CFLAGS)"
PQ_CGO_LDFLAGS = CGO_LDFLAGS="$(PQ_LDFLAGS)"

# Build targets
.PHONY: all clean liboqs liboqs-clean pq-test pq-bench install-deps

all: liboqs geth

# Install system dependencies for building liboqs
install-deps:
	@echo "Installing dependencies for liboqs..."
	@if command -v apt-get >/dev/null 2>&1; then \
		sudo apt-get update && \
		sudo apt-get install -y cmake ninja-build libssl-dev; \
	elif command -v yum >/dev/null 2>&1; then \
		sudo yum install -y cmake ninja-build openssl-devel; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install cmake ninja openssl; \
	else \
		echo "Please install cmake, ninja-build, and openssl development libraries manually"; \
		exit 1; \
	fi

# Download and build liboqs
liboqs: $(LIBOQS_INSTALL_DIR)/lib/liboqs.a

$(LIBOQS_INSTALL_DIR)/lib/liboqs.a:
	@echo "Building liboqs $(LIBOQS_VERSION)..."
	@mkdir -p $(LIBOQS_DIR)
	@if [ ! -d "$(LIBOQS_DIR)/liboqs-$(LIBOQS_VERSION)" ]; then \
		cd $(LIBOQS_DIR) && \
		curl -L https://github.com/open-quantum-safe/liboqs/archive/$(LIBOQS_VERSION).tar.gz | tar xz; \
	fi
	@mkdir -p $(LIBOQS_BUILD_DIR)
	@cd $(LIBOQS_BUILD_DIR) && \
		cmake -GNinja \
			-DCMAKE_INSTALL_PREFIX=$(LIBOQS_INSTALL_DIR) \
			-DCMAKE_BUILD_TYPE=Release \
			-DBUILD_SHARED_LIBS=OFF \
			-DOQS_BUILD_ONLY_LIB=ON \
			-DOQS_MINIMAL_BUILD="SIG_dilithium_2;SIG_dilithium_3;SIG_dilithium_5;KEM_kyber_512;KEM_kyber_768;KEM_kyber_1024" \
			-DOQS_USE_OPENSSL=ON \
			../liboqs-$(LIBOQS_VERSION) && \
		ninja && \
		ninja install
	@echo "liboqs built successfully"

# Clean liboqs build
liboqs-clean:
	@echo "Cleaning liboqs..."
	@rm -rf $(LIBOQS_BUILD_DIR) $(LIBOQS_INSTALL_DIR)

# Build geth with post-quantum support
geth: liboqs
	@echo "Building geth with post-quantum support..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go build -ldflags "-s -w" -o build/bin/geth ./cmd/geth

# Build all tools with post-quantum support
all-tools: liboqs
	@echo "Building all tools with post-quantum support..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go build -ldflags "-s -w" -o build/bin/geth ./cmd/geth
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go build -ldflags "-s -w" -o build/bin/bootnode ./cmd/bootnode
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go build -ldflags "-s -w" -o build/bin/evm ./cmd/evm
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go build -ldflags "-s -w" -o build/bin/rlpdump ./cmd/rlpdump

# Run post-quantum tests
pq-test: liboqs
	@echo "Running post-quantum tests..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go test -v ./crypto/mldsa/...
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go test -v ./core/vm/... -run TestPQ
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go test -v ./consensus/clique/... -run TestPQ

# Run post-quantum benchmarks
pq-bench: liboqs
	@echo "Running post-quantum benchmarks..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go test -bench=. -benchmem ./crypto/mldsa/...

# Validate liboqs installation
validate-liboqs:
	@echo "Validating liboqs installation..."
	@if [ -f "$(LIBOQS_INSTALL_DIR)/lib/liboqs.a" ]; then \
		echo "✓ liboqs static library found"; \
	else \
		echo "✗ liboqs static library not found"; \
		exit 1; \
	fi
	@if [ -f "$(LIBOQS_INSTALL_DIR)/include/oqs/oqs.h" ]; then \
		echo "✓ liboqs headers found"; \
	else \
		echo "✗ liboqs headers not found"; \
		exit 1; \
	fi
	@echo "liboqs validation successful"

# Create a minimal test to verify ML-DSA support
test-mldsa: liboqs
	@echo "Testing ML-DSA support..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go run -ldflags "-s -w" ./crypto/mldsa/cmd/test-mldsa.go

# Docker build with post-quantum support
docker-pq:
	@echo "Building Docker image with post-quantum support..."
	@docker build -f Dockerfile.pq -t splendor-pq:latest .

# Clean all build artifacts
clean: liboqs-clean
	@echo "Cleaning build artifacts..."
	@rm -rf build/bin/*
	@go clean -cache

# Development setup
dev-setup: install-deps liboqs validate-liboqs
	@echo "Development environment setup complete"
	@echo "You can now build with: make geth"
	@echo "Run tests with: make pq-test"

# Production build (optimized)
production: liboqs
	@echo "Building production binaries with post-quantum support..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go build \
		-ldflags "-s -w -X main.gitCommit=$(shell git rev-parse HEAD) -X main.buildDate=$(shell date -u +%Y%m%d)" \
		-tags netgo -installsuffix netgo \
		-o build/bin/geth ./cmd/geth
	@strip build/bin/geth 2>/dev/null || true
	@echo "Production build complete"

# Static analysis for post-quantum code
lint-pq:
	@echo "Running static analysis on post-quantum code..."
	@golangci-lint run ./crypto/mldsa/...
	@golangci-lint run ./core/vm/contracts_pq.go
	@golangci-lint run ./consensus/clique/clique_pq.go
	@golangci-lint run ./params/pq_config.go

# Security audit helpers
security-audit:
	@echo "Running security audit tools..."
	@gosec ./crypto/mldsa/...
	@gosec ./core/vm/contracts_pq.go
	@gosec ./consensus/clique/clique_pq.go

# Generate test vectors (placeholder for NIST test vectors)
generate-test-vectors:
	@echo "Generating/downloading NIST test vectors..."
	@mkdir -p testdata/nist-vectors
	@echo "TODO: Download official NIST ACVP test vectors for ML-DSA"
	@echo "See: https://csrc.nist.gov/Projects/Cryptographic-Algorithm-Validation-Program"

# Performance profiling
profile-pq: liboqs
	@echo "Running performance profiling..."
	@$(PQ_CGO_CFLAGS) $(PQ_CGO_LDFLAGS) go test -cpuprofile=cpu.prof -memprofile=mem.prof -bench=. ./crypto/mldsa/...
	@echo "Profiles saved as cpu.prof and mem.prof"
	@echo "View with: go tool pprof cpu.prof"

# Help target
help:
	@echo "Splendor Post-Quantum Build System"
	@echo ""
	@echo "Targets:"
	@echo "  install-deps     - Install system dependencies"
	@echo "  liboqs          - Build liboqs library"
	@echo "  geth            - Build geth with PQ support"
	@echo "  all-tools       - Build all tools with PQ support"
	@echo "  pq-test         - Run post-quantum tests"
	@echo "  pq-bench        - Run post-quantum benchmarks"
	@echo "  validate-liboqs - Validate liboqs installation"
	@echo "  dev-setup       - Complete development setup"
	@echo "  production      - Optimized production build"
	@echo "  clean           - Clean all build artifacts"
	@echo "  help            - Show this help"
	@echo ""
	@echo "Environment Variables:"
	@echo "  LIBOQS_VERSION  - liboqs version to build (default: $(LIBOQS_VERSION))"
	@echo "  LIBOQS_DIR      - liboqs directory (default: $(LIBOQS_DIR))"

# Default target
.DEFAULT_GOAL := help
