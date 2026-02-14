.PHONY: build run

BINARY_PATH=./bin/app

build:
	go build -buildvcs=false -o $(BINARY_PATH) ./cmd/app

run: build
	$(BINARY_PATH)

test:
	go test -v ./...