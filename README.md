# gopbx

`gopbx` 是一个基于 Echo 的实时语音交互系统

当前骨架已包含：

- Echo 启动入口与路由装配
- WebSocket 首包 `invite/accept` 校验骨架
- `/call` `/call/webrtc` `/call/lists` `/call/kill/:id` `/iceservers` `/llm/v1/*` 路由
- 会话管理器、协议兼容常量、配置加载器
- 合同测试、部署脚本与文档占位

启动示例：

```bash
go run ./cmd/gateway -config configs/config.dev.yaml
```
