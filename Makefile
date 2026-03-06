APP_NAME := gopbx
CONFIG ?= configs/config.dev.yaml

.PHONY: run fmt test tidy

run:
	go run ./cmd/gateway -config $(CONFIG)

fmt:
	gofmt -w ./cmd ./internal ./pkg ./sdk ./test

test:
	go test ./...

tidy:
	go mod tidy
