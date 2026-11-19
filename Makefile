.PHONY: run test test-api bruno-install migrate-care-journeys migrate-flexible-programs build fmt vet up down

run:
	go run ./cmd/api

test:
	go test ./...

bruno-install:
	npm ci

test-api:
	npm run test:api

migrate-care-journeys:
	docker compose exec -T postgres psql -U postgres -d example_company < migrations/000002_care_journeys.up.sql

migrate-flexible-programs:
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d example_company -1 < migrations/000003_flexible_programs.up.sql

build:
	go build ./cmd/api

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

up:
	docker compose up --build

down:
	docker compose down
