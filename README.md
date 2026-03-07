# gopbx

`gopbx` 是一个基于 Echo 的实时语音交互系统。

当前配置管理使用 `koanf`，并且所有运行参数统一从环境变量读取。

当前骨架已包含：

- Echo 启动入口与路由装配
- WebSocket 首包 `invite/accept` 校验骨架
- `/call` `/call/webrtc` `/call/lists` `/call/kill/:id` `/iceservers` `/llm/v1/*` 路由
- 会话管理器、协议兼容常量、配置加载器
- 合同测试、部署脚本与文档占位

启动示例：

```bash
set -a
source configs/gopbx.env.example
set +a
go run ./cmd/gateway
```

核心环境变量：

- `GOPBX_SERVER__ADDRESS`：服务监听地址
- `GOPBX_SERVER__SHUTDOWN_TIMEOUT`：优雅停机超时
- `GOPBX_LLM_PROXY__ENDPOINT`：LLM 上游地址
- `GOPBX_LLM_PROXY__API_KEY`：LLM 上游密钥
- `GOPBX_ICE_PROVIDER__ENDPOINT`：远端 ICE 分配接口地址
- `GOPBX_ICE_PROVIDER__API_KEY`：远端 ICE 分配接口密钥
- `GOPBX_ICE_PROVIDER__TIMEOUT`：远端 ICE 分配接口超时
- `GOPBX_RECORDER_PATH`：会话录音与 command/event dump 文件目录
- `GOPBX_ICE_SERVERS`：ICE Server JSON 数组
