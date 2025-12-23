.PHONY: run infra-up infra-down test clean run-api run-worker

APP_ENV ?= dev
DOCKER_COMPOSE_FILE = docker-compose.yml

# Run tests with Verbose output (-v) and Coverage (-cover)
# -v: Prints the name of every test and how long it took.
# -coverpkg=./internal/...: Tells Go to calculate coverage for your actual code, not just the test file.
test:
	APP_ENV=dev go test -v -coverpkg=./internal/... ./tests/integration/...

# Generate a visual HTML Coverage Report
# 1. Runs tests and saves raw data to 'coverage.out'
# 2. Converts 'coverage.out' to 'coverage.html'
# 3. Opens it (on Mac/Linux) or tells you where it is.
test-html:
	APP_ENV=dev go test -v -coverpkg=./internal/... -coverprofile=coverage.out ./tests/integration/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Report generated: coverage.html"
	@echo "   Open this file in your browser to see exactly which lines of code are covered."

infra-up:
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d

infra-down:
	docker compose -f $(DOCKER_COMPOSE_FILE) down

# Run the API Server
run-api:
	go run cmd/api-server/main.go

# Run the Background Worker
run-worker:
	go run cmd/worker/main.go


test:
	APP_ENV=$(APP_ENV) go test ./tests/integration/...

build:
	go build -o bin/api-server cmd/api-server/main.go

clean:
	go clean
	rm -rf bin/

fmt:
	go fmt ./...

vet:
	go vet ./...
