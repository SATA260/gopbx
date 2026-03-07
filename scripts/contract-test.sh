#!/usr/bin/env bash
set -euo pipefail

# 这个脚本只运行契约回归，用来快速验证 HTTP/WS 协议没有漂移。
go test ./test/contract/...
