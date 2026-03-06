# Migration Guide

本项目作为独立语音网关运行，现有 `go-zero` 调用方只需切换 HTTP/WS 地址即可。

语音厂商能力已收敛为 Aliyun-only，迁移调用方时应把 `provider` 配置统一为 `aliyun`。
