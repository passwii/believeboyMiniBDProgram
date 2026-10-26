.PHONY: up down psql migrate sqlc test check

up:
	docker compose --env-file .env -f infra/compose.yaml up -d

down:
	docker compose --env-file .env -f infra/compose.yaml down

psql:
	docker compose --env-file .env -f infra/compose.yaml exec postgres psql -U believeboy -d believeboy

migrate:
	cd services/api && go run ./cmd/migrate up

sqlc:
	cd services/api && sqlc generate

test:
	cd services/api && go test ./...

check:
	cd services/api && gofmt -w . && go vet ./... && go test ./...
