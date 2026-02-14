.PHONY: build run

BINARY_PATH=./bin/app

# -buildvcs=false
build:
	go build -o $(BINARY_PATH) ./cmd/app

run: build
	$(BINARY_PATH)

test:
	go test -v ./...

compose-integration-test:
	docker compose --profile test up --build --abort-on-container-exit

compose-load-test:
	docker compose --profile load run --rm loadtest \
  -dsn "postgres://scanblock@postgres_test:5432/scanblock_test?sslmode=disable" \
  -dur 2m -avg-rps 400 -peak-rps 2000 -ramp 20s -rw 15 -workers 128
