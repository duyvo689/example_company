.PHONY: run test test-api bruno-install build fmt vet up down

run:
	go run ./cmd/api

test:
	go test ./...

bruno-install:
	npm ci

test-api:
	npm run test:api

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
