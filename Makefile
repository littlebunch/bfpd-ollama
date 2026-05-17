.PHONY: help build run-api run-cli seed-vectors install-models deps test clean docker-up docker-down

help:
	@echo "Available targets:"
	@echo "  build             - Build all binaries"
	@echo "  run-api           - Run API server"
	@echo "  run-cli           - Run CLI chat client"
	@echo "  seed-vectors      - Seed vector database from MySQL"
	@echo "  install-models    - Pull LLM models from Ollama"
	@echo "  deps              - Install Go dependencies"
	@echo "  docker-up         - Start Docker services (Ollama + Qdrant)"
	@echo "  docker-down       - Stop Docker services"
	@echo "  test              - Run tests"
	@echo "  clean             - Remove binaries"

build: build-api build-cli build-seed

build-api:
	go build -o bin/api ./cmd/api

build-cli:
	go build -o bin/chat-cli ./cmd/chat-cli

build-seed:
	go build -o bin/seed-vectors ./cmd/seed-vectors

run-api: build-api
	./bin/api

run-cli: build-cli
	./bin/chat-cli --rag

seed-vectors: build-seed
	./bin/seed-vectors

install-models:
	@echo "Pulling neural-chat model..."
	ollama pull neural-chat
	@echo "Pulling embedding model..."
	ollama pull nomic-embed-text

deps:
	go mod download
	go mod tidy

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

test:
	go test -v ./...

clean:
	rm -rf bin/
	go clean

ollama-status:
	@echo "Checking Ollama status..."
	@curl -s http://localhost:11434/api/tags | jq . || echo "❌ Ollama not running"

# ========== BUILD & RUN ==========

build:
	@echo "Building Go binary..."
	cd /Users/gmoore/repos/bfpd-ollama && \
	go build -o import_gbfpd import_gbfpd.go
	@echo "✓ Build successful: ./import_gbfpd"

run: build
	@echo "Running BFPD importer..."
	@echo "Expected time: ~6 minutes for 34.4M rows"
	@echo ""
	cd /Users/gmoore/repos/bfpd-ollama && \
	./import_gbfpd 2>&1 | tee import_run.log
	@echo ""
	@echo "Import complete. Summary saved to: import_run.log"

quick-test: build
	@echo "Running quick test on subset of data..."
	@echo "This should complete in < 30 seconds"
	cd /Users/gmoore/repos/bfpd-ollama && \
	./import_gbfpd 2>&1 | tail -30

# ========== TESTING ==========

test-ollama:
	@echo "Testing Ollama connection..."
	@curl -s http://localhost:11434/api/tags | jq .models[].name || \
	  (echo "❌ Ollama not running. Start with: make ollama-start" && exit 1)

test-mysql:
	@echo "Testing MySQL connection..."
	@mysql -ugmoore -pmaggie2pie -e "SELECT VERSION();" || \
	  (echo "❌ MySQL not running or credentials invalid" && exit 1)

test-csv:
	@echo "Checking CSV files..."
	@ls -lh /Users/gmoore/bfpd/2025-12/FoodData_Central_branded_food_csv_2025-12-18/ | head -20

test: test-ollama test-mysql test-csv
	@echo "✓ All pre-flight checks passed"

# ========== CLEANUP ==========

clean:
	@echo "Cleaning up..."
	cd /Users/gmoore/repos/bfpd-ollama && \
	rm -f import_gbfpd
	@echo "✓ Cleanup complete"

clean-db:
	@echo "Dropping database gbfpd..."
	@mysql -ugmoore -pmaggie2pie -e "DROP DATABASE IF EXISTS gbfpd;"
	@echo "✓ Database dropped"

# ========== DEVELOPMENT ==========

fmt:
	@echo "Formatting Go code..."
	gofmt -w -s /Users/gmoore/repos/bfpd-ollama/import_gbfpd.go
	@echo "✓ Formatted"

lint:
	@echo "Running Go lint..."
	golint /Users/gmoore/repos/bfpd-ollama/import_gbfpd.go || echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

vet:
	@echo "Running Go vet..."
	cd /Users/gmoore/repos/bfpd-ollama && \
	go vet ./...

# ========== DOCUMENTATION ==========

docs:
	@echo "Generating documentation..."
	@echo ""
	@echo "Project Context: .continue/project-context.md"
	@echo "Code Rules: .continue/rules.md"
	@echo "Configuration: .continue/config.json"
	@echo ""
	@cat .continue/rules.md | head -50

# ========== CONTINUE.DEV SETUP ==========

continue-config:
	@echo "Continue.dev Configuration:"
	@echo "1. Install VS Code extension: Continue"
	@echo "2. Configuration file: ~/.continue/config.json"
	@echo "3. Project context: .continue/project-context.md"
	@echo "4. Code rules: .continue/rules.md"
	@echo ""
	@echo "Features:"
	@echo "  - Tab autocomplete: Start typing, Mistral suggests"
	@echo "  - Slash commands: /edit, /comment, /doc, /explain"
	@echo "  - Custom commands: /test, /refactor"
	@echo ""
	@cat .continue/config.json

# ========== STATS & MONITORING ==========

stats:
	@echo "BFPD Importer Statistics:"
	@echo "========================="
	@echo ""
	@mysql -ugmoore -pmaggie2pie gbfpd -e "SELECT table_name, table_rows FROM information_schema.tables WHERE table_schema='gbfpd' ORDER BY table_rows DESC;" 2>/dev/null || echo "Database not available"

monitor:
	@echo "Monitoring import process (refresh every 5 seconds)..."
	@while true; do \
	  clear; \
	  echo "BFPD Importer - Real-time Monitoring"; \
	  echo "==================================="; \
	  echo ""; \
	  echo "Memory Usage:"; \
	  ps aux | grep import_gbfpd | grep -v grep | awk '{print "  RSS: " $$6 "KB, VSZ: " $$5 "KB"}'; \
	  echo ""; \
	  echo "Row Counts:"; \
	  mysql -ugmoore -pmaggie2pie gbfpd -e "SELECT table_name, table_rows FROM information_schema.tables WHERE table_schema='gbfpd' ORDER BY table_rows DESC LIMIT 5;" 2>/dev/null; \
	  echo ""; \
	  sleep 5; \
	done

# ========== QUICK START ==========

init: ollama-install model-pull
	@echo ""
	@echo "✓ Ollama setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "1. Start Ollama server: make ollama-start (in new terminal)"
	@echo "2. Build the importer: make build"
	@echo "3. Run the import: make run"

info:
	@echo "BFPD Importer Project Information"
	@echo "===================================="
	@echo ""
	@echo "Project:"
	@echo "  Type: CSV Importer (Go)"
	@echo "  Location: /Users/gmoore/repos/bfpd-ollama"
	@echo "  Main file: import_gbfpd.go"
	@echo ""
	@echo "Data:"
	@echo "  Source: USDA FoodData Central CSV (2025-12)"
	@echo "  Total rows: ~34.4 million"
	@echo "  Files: 11 CSV files"
	@echo "  Size: ~1.5GB total"
	@echo ""
	@echo "Database:"
	@echo "  Host: localhost"
	@echo "  User: gmoore"
	@echo "  Database: gbfpd"
	@echo ""
	@echo "Local LLM Setup:"
	@echo "  Model: Mistral 7B"
	@echo "  Framework: Ollama"
	@echo "  IDE Integration: Continue.dev (VS Code)"
	@echo "  Config: .continue/"
	@echo ""
	@echo "Performance:"
	@echo "  Import time: ~360 seconds (6 minutes)"
	@echo "  Throughput: ~95,000 rows/second"
	@echo "  Memory: ~8GB (for Ollama model)"
