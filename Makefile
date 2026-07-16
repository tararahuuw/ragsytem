.PHONY: help run build tidy swag test docker-up docker-down install-tools

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

run: ## Run the server (go run)
	go run .

build: ## Build the binary into ./bin
	go build -o bin/ragsystem .

tidy: ## Sync go.mod / go.sum
	go mod tidy

swag: ## Regenerate Swagger docs into ./docs
	swag init -g main.go -o docs --parseDependency --parseInternal

test: ## Run all tests
	go test ./...

docker-up: ## Start local Postgres via docker compose
	docker compose up -d

docker-down: ## Stop local infra
	docker compose down

install-tools: ## Install dev tooling (swag)
	go install github.com/swaggo/swag/cmd/swag@latest
