.PHONY: build run

APP_BINARY_PATH=./bin/app

build:
	go build -buildvcs=false -o ${APP_BINARY_PATH} ./cmd/app

run: build
	$(APP_BINARY_PATH)

