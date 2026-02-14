FROM golang:1.24.2-alpine3.21 AS build

USER 0

WORKDIR /scanblock
COPY go.mod go.sum ./
RUN apk add g++ && apk add make && \
    go mod download
COPY . .
RUN make build

FROM build AS test
RUN make test

FROM alpine:3.21 AS executable

WORKDIR /scanblock
COPY --chown=1001:0 --from=build /scanblock/bin/app ./app

CMD ["./app"]