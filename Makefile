SERVICES := identity core payment notification reporting

.PHONY: up down build test fmt vet tidy migrate-up migrate-down

## Start the full local stack (Postgres, RabbitMQ, Redis, all services, gateway).
up:
	docker compose -f deployments/docker/docker-compose.yml up -d --build

## Stop and remove the local stack.
down:
	docker compose -f deployments/docker/docker-compose.yml down

## Build every module (shared, all services, gateway).
build:
	@for d in shared services/identity services/core services/payment services/notification services/reporting gateway; do \
		echo "==> $$d"; (cd $$d && go build ./...) || exit 1; \
	done

## Run tests for every module.
test:
	@for d in shared services/identity services/core services/payment services/notification services/reporting gateway; do \
		echo "==> $$d"; (cd $$d && go test ./...) || exit 1; \
	done

## Format check every module (fails if gofmt would change anything).
fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:" && gofmt -l . && exit 1)

## Vet every module.
vet:
	@for d in shared services/identity services/core services/payment services/notification services/reporting gateway; do \
		echo "==> $$d"; (cd $$d && go vet ./...) || exit 1; \
	done

## go mod tidy every module.
tidy:
	@for d in shared services/identity services/core services/payment services/notification services/reporting gateway; do \
		echo "==> $$d"; (cd $$d && go mod tidy) || exit 1; \
	done

## Apply migrations for one service: make migrate-up SERVICE=core
migrate-up:
	@test -n "$(SERVICE)" || (echo "usage: make migrate-up SERVICE=<name>"; exit 1)
	cd services/$(SERVICE) && go run ./cmd/migrate up

## Roll back the last migration for one service: make migrate-down SERVICE=core
migrate-down:
	@test -n "$(SERVICE)" || (echo "usage: make migrate-down SERVICE=<name>"; exit 1)
	cd services/$(SERVICE) && go run ./cmd/migrate down
