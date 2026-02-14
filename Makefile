.PHONY: build run

BINARY_PATH=./bin/app

# -buildvcs=false
build:
	go build -o $(BINARY_PATH) ./cmd/app

run: build
	$(BINARY_PATH)

test:
	go test -v ./...
