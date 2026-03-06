# WS Protocol

首包必须是 `invite` 或 `accept`，事件名与字段名遵循 `rustvoice-old` 兼容约束。

当前骨架默认只支持 `provider=aliyun` 的 ASR/TTS 方向；协议字段仍保留 `provider`，用于兼容旧报文结构。
