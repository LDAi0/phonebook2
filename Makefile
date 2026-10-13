.PHONY: up down logs test tidy run-api run-consumer migrate-up migrate-down

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api consumer

test:
	go test ./... -race -count=1

tidy:
	go mod tidy

run-api:
	go run ./cmd/server

run-consumer:
	go run ./cmd/consumer

migrate-up:
	docker compose run --rm migrate

migrate-down:
	docker compose run --rm migrate -path=/migrations -database=postgres://phonebook:phonebook@postgres:5432/phonebook?sslmode=disable down 1
