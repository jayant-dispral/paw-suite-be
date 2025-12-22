.PHONY: run infra-up infra-down test clean

APP_ENV ?= dev
DOCKER_COMPOSE_FILE = docker-compose.yml

run:
	APP_ENV=$(APP_ENV) go run cmd/api-server/main.go

infra-up:
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d

infra-down:
	docker compose -f $(DOCKER_COMPOSE_FILE) down

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