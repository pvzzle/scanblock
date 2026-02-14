FROM golang:1.24.2-alpine3.21 AS base

USER 0
WORKDIR /scanblock

# deps
COPY go.mod go.sum ./
RUN apk add --no-cache g++ make && go mod download

# source
COPY . .

# ---------- build app binary ----------
FROM base AS build
RUN make build

# ---------- unit tests runtime ----------
FROM base AS test-unit
CMD ["sh", "-lc", "go test ./..."]

# ---------- integration tests runtime ----------
FROM base AS test-integration
CMD ["sh", "-lc", "go test -tags=integration ./..."]

# ---------- loadtest binary ----------
FROM base AS loadtest-build
RUN go build -o /usr/local/bin/loadtest ./cmd/loadtest

FROM alpine:3.21 AS loadtest
COPY --from=loadtest-build /usr/local/bin/loadtest /usr/local/bin/loadtest
ENTRYPOINT ["loadtest"]

# ---------- final executable ----------
FROM alpine:3.21 AS executable
WORKDIR /scanblock
COPY --chown=1001:0 --from=build /scanblock/bin/app ./app
CMD ["./app"]
