# Silero 模型目录

把 Silero 的 ONNX 模型放到：

- `third_party/models/silero_vad.onnx`

当前项目默认会通过环境变量 `GOPBX_VAD_NATIVE_MODEL_PATH` 读取模型路径。

如果你直接使用 `./scripts/native-vad.sh run`，它会优先把该环境变量指向这个默认位置。
