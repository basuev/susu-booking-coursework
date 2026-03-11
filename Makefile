.PHONY: build run test lint clean compose-up compose-down proto migrate-up migrate-down

APP_NAME := booking-svc
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/booking-svc

run: build
	./$(BUILD_DIR)/$(APP_NAME)

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

compose-up:
	docker compose up -d

compose-down:
	docker compose down

proto:
	buf generate

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down
