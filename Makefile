APP_NAME := gopbx
ENV_FILE ?= configs/gopbx.env.example

.PHONY: run fmt test tidy

run:
	set -a && . $(ENV_FILE) && set +a && go run ./cmd/gateway

fmt:
	gofmt -w ./cmd ./internal ./pkg ./sdk ./test

test:
	go test ./...

tidy:
	go mod tidy
