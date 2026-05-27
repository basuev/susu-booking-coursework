.PHONY: build run dev test test-unit test-integration coverage lint fmt clean compose-up compose-down proto migrate-up migrate-down

APP_NAME := booking-svc
BUILD_DIR := bin
DATABASE_URL ?= postgres://booking:booking@localhost:5432/booking?sslmode=disable

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/booking-svc

run: build
	./$(BUILD_DIR)/$(APP_NAME)

dev: compose-up
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/booking-svc

test:
	go test -race ./...

test-unit:
	go test -race ./...

test-integration:
	go test -race -tags=integration ./...

coverage:
	go test -race -coverpkg=./internal/...,./pkg/... -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

fmt:
	gofumpt -l -w .
	goimports -w .

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

compose-up:
	docker compose up -d

compose-down:
	docker compose down

proto:
	buf generate

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down
