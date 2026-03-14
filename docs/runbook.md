# Runbook

## 本地启动

```bash
set -a
source configs/gopbx.env.example
set +a
go run ./cmd/gateway
```

如果你要启用本地 `cgo + Silero + ONNX Runtime`：

```bash
./scripts/native-vad.sh run
```

## 常用测试命令

- 合同测试：`go test ./test/contract/...`
- 集成测试：`go test ./test/integration/...`
- 端到端测试：`go test ./test/e2e/...`
- 全量测试：`go test ./...`

也可以直接执行：

```bash
./scripts/contract-test.sh
./scripts/test.sh
```

## 关键运行产物

- dump 文件目录：`GOPBX_RECORDER_PATH`
- dump 文件名：`<session_id>.events.jsonl`
- 话单文件名：`<session_id>.call.json`

## 排障关注点

### 首包失败

- 检查首包是否为 `invite` 或 `accept`
- 查看客户端是否收到 `event=error`
- 查看 dump 文件里首条 `command` 和返回的 `event`

### 会话未出现在 `/call/lists`

- 确认是否已经发出 `answer`
- 确认会话是否已进入 `closing/closed/failed`
- 检查是否被 `/call/kill/{id}` 提前结束

### 没有 `asrFinal`

- 确认连接走的是 `/call` 而不是 `/call/webrtc`
- 确认客户端是否发送了二进制音频帧
- 检查 dump 文件中是否有对应 `command/event`

### `tts/play` 后没有结束事件

- 检查是否收到 `trackStart`
- 检查 `metrics` 和 `trackEnd` 是否在 dump 文件中出现
- 如果启用了 `autoHangup`，确认最后是否收到 `hangup`

### 话单未写入

- 检查 `GOPBX_CALLRECORD__TYPE`
- 本地后端检查 `GOPBX_RECORDER_PATH`
- HTTP/S3 后端检查对应 endpoint、bucket、prefix、api key 和 timeout

## 文档索引

- 总体迁移说明：`docs/migration-overview.md`
- Native Silero VAD：`docs/native-vad.md`
- 可执行计划：`tmp/gopbx-executable-plan.md`
