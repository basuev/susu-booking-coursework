.PHONY: build run test lint clean

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
