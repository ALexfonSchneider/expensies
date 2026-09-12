.PHONY: build run test lint tidy clean web-install web-build web-dev db-up db-down docker-build docker-up docker-down docker-logs

build: ## Go binary with the embedded frontend (run web-build first)
	go build -o bin/expenses ./cmd

run:
	go run ./cmd

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	go clean -cache -testcache
	rm -rf bin/

# Frontend targets expect npm on PATH (inside WSL on this machine).
web-install:
	cd web && npm install

web-build:
	cd web && npm run build

web-dev:
	cd web && npm run dev

# Postgres from docker-compose.yaml (port 5433).
db-up:
	docker compose up -d

db-down:
	docker compose down

# Whole stack in Docker. The image build needs ../platforme next to this
# directory (go.mod replace); compose already sets the context.
docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app
