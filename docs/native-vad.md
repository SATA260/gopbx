# Native Silero VAD

## 放置位置

如果你已经下载好了 ONNX Runtime，按下面放：

- `onnxruntime_c_api.h` 放到 `third_party/onnxruntime/include/`
- `libonnxruntime.so` 放到 `third_party/onnxruntime/lib/`
- `silero_vad.onnx` 放到 `third_party/models/silero_vad.onnx`

如果你的包里只有 `libonnxruntime.so.<版本号>`，也没关系，`scripts/native-vad.sh` 会自动补一个 `libonnxruntime.so` 软链接。

如果你下载的是 ONNX Runtime 官方 Linux 包，通常解压后会自带：

- `include/`
- `lib/`

你可以直接把这两个目录整体复制到：

- `third_party/onnxruntime/include/`
- `third_party/onnxruntime/lib/`

## 建议目录结构

```text
third_party/
  onnxruntime/
    include/
      onnxruntime_c_api.h
      ...
    lib/
      libonnxruntime.so
      ...
  models/
    silero_vad.onnx
```

## 一个脚本直接跑

项目里现在统一用：`scripts/native-vad.sh`

它会自动：

- 查找 `third_party/onnxruntime` 下的头文件和动态库
- 查找 `third_party/models` 下的 `onnx` 模型
- 自动补 `libonnxruntime.so` 软链接
- 自动选择本机 `cc/c++/gcc/g++/clang/clang++`
- 设置 `CGO_CFLAGS`、`CGO_LDFLAGS`、`LD_LIBRARY_PATH`
- 用 `silero_native` tag 启动或测试

## 自检

直接看当前识别到的环境：

```bash
./scripts/native-vad.sh env
```

只做 native 路径自检：

```bash
./scripts/native-vad.sh check
```

它会自动编译并测试 `internal/adapter/outbound/vad`

## 启动服务

```bash
./scripts/native-vad.sh run
```

## invite 配置

推荐这样传：

```json
{
  "vad": {
    "type": "hybrid",
    "endpoint": "native://"
  }
}
```

也可以直接显式写模型路径：

```json
{
  "vad": {
    "type": "hybrid",
    "endpoint": "native:///absolute/path/to/silero_vad.onnx"
  }
}
```

## 常见问题

### 找不到 `onnxruntime_c_api.h`

说明头文件没放到：`third_party/onnxruntime/include/`

### 找不到 `libonnxruntime.so`

说明动态库没放到：`third_party/onnxruntime/lib/`

### 运行时报 `libonnxruntime.so: cannot open shared object file`

说明动态库路径没被正确带上，优先直接用脚本启动：

```bash
./scripts/native-vad.sh run
```

### 默认不走 native

说明你没有带构建 tag，优先直接用脚本：

```bash
./scripts/native-vad.sh run
```
