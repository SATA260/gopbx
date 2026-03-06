#!/usr/bin/env bash
set -euo pipefail

gofmt -w ./cmd ./internal ./pkg ./sdk ./test
go vet ./...
