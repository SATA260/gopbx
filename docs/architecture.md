# Architecture

`gopbx` 采用 Echo + SessionManager + Media Pipeline 的分层结构，对外保持 rustvoice-old 的接口兼容。

当前第三方语音能力按 Aliyun-only 设计：ASR/TTS provider 层只保留 `aliyun`，后续真实实现也以阿里云协议为唯一目标。
