.PHONY: build run dev test lint migrate-up migrate-down docker-up docker-down clean

# Build
build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

# Run locally
run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# Development with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev-api:
	air -c .air.api.toml

dev-worker:
	air -c .air.worker.toml

# Tests
test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Lint
lint:
	golangci-lint run

# Migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-reset:
	migrate -path migrations -database "$(DATABASE_URL)" drop -f
	migrate -path migrations -database "$(DATABASE_URL)" up

# Docker
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f

docker-reset:
	docker-compose down -v
	docker-compose up -d

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out

# Install tools
tools:
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
