#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-run}"
if [[ $# -gt 0 ]]; then
  shift
fi

usage() {
  cat <<'EOF'
用法：
  ./scripts/native-vad.sh run        启动带 native Silero VAD 的网关
  ./scripts/native-vad.sh check      检查 native 依赖并编译测试 VAD 包
  ./scripts/native-vad.sh test       运行全量测试（带 silero_native tag）
  ./scripts/native-vad.sh env        打印当前检测到的 native 环境

可选环境变量：
  ENV_FILE                  默认 configs/gopbx.env.example
  ONNXRUNTIME_DIR           ONNX Runtime 根目录
  ONNXRUNTIME_INCLUDE_DIR   头文件目录
  ONNXRUNTIME_LIB_DIR       动态库目录
  GOPBX_VAD_NATIVE_MODEL_PATH  Silero onnx 模型路径
  CC / CXX                  C/C++ 编译器，默认自动选择 cc/c++/gcc/g++/clang/clang++
EOF
}

pick_compiler() {
  local current="${1:-}"
  shift || true
  if [[ -n "$current" ]]; then
    printf '%s\n' "$current"
    return 0
  fi
  local candidate
  for candidate in "$@"; do
    if command -v "$candidate" >/dev/null 2>&1; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

pick_go() {
  local current="${GO_BIN:-}"
  if [[ -n "$current" ]]; then
    printf '%s\n' "$current"
    return 0
  fi
  local candidate
  for candidate in go /usr/local/go/bin/go /usr/bin/go /snap/bin/go; do
    if command -v "$candidate" >/dev/null 2>&1; then
      command -v "$candidate"
      return 0
    fi
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

find_header_dir() {
  if [[ -n "${ONNXRUNTIME_INCLUDE_DIR:-}" ]]; then
    printf '%s\n' "$ONNXRUNTIME_INCLUDE_DIR"
    return 0
  fi
  if [[ -n "${ONNXRUNTIME_DIR:-}" && -f "$ONNXRUNTIME_DIR/include/onnxruntime_c_api.h" ]]; then
    printf '%s\n' "$ONNXRUNTIME_DIR/include"
    return 0
  fi
  if [[ -f "$ROOT_DIR/third_party/onnxruntime/include/onnxruntime_c_api.h" ]]; then
    printf '%s\n' "$ROOT_DIR/third_party/onnxruntime/include"
    return 0
  fi
  local found
  found="$(find "$ROOT_DIR/third_party/onnxruntime" -type f -name onnxruntime_c_api.h 2>/dev/null | head -n 1 || true)"
  if [[ -n "$found" ]]; then
    dirname "$found"
    return 0
  fi
  return 1
}

find_lib_dir() {
  if [[ -n "${ONNXRUNTIME_LIB_DIR:-}" ]]; then
    printf '%s\n' "$ONNXRUNTIME_LIB_DIR"
    return 0
  fi
  if [[ -n "${ONNXRUNTIME_DIR:-}" && -d "$ONNXRUNTIME_DIR/lib" ]]; then
    printf '%s\n' "$ONNXRUNTIME_DIR/lib"
    return 0
  fi
  if [[ -d "$ROOT_DIR/third_party/onnxruntime/lib" ]]; then
    printf '%s\n' "$ROOT_DIR/third_party/onnxruntime/lib"
    return 0
  fi
  local found
  found="$(find "$ROOT_DIR/third_party/onnxruntime" -type f \( -name 'libonnxruntime.so' -o -name 'libonnxruntime.so.*' \) 2>/dev/null | head -n 1 || true)"
  if [[ -n "$found" ]]; then
    dirname "$found"
    return 0
  fi
  return 1
}

ensure_unversioned_so() {
  local lib_dir="$1"
  if [[ -f "$lib_dir/libonnxruntime.so" ]]; then
    return 0
  fi
  local versioned
  versioned="$(find "$lib_dir" -maxdepth 1 -type f -name 'libonnxruntime.so.*' | head -n 1 || true)"
  if [[ -n "$versioned" ]]; then
    ln -sf "$(basename "$versioned")" "$lib_dir/libonnxruntime.so"
  fi
}

find_model_path() {
  if [[ -n "${GOPBX_VAD_NATIVE_MODEL_PATH:-}" ]]; then
    printf '%s\n' "$GOPBX_VAD_NATIVE_MODEL_PATH"
    return 0
  fi
  if [[ -f "$ROOT_DIR/third_party/models/silero_vad.onnx" ]]; then
    printf '%s\n' "$ROOT_DIR/third_party/models/silero_vad.onnx"
    return 0
  fi
  local found
  found="$(find "$ROOT_DIR/third_party/models" -type f -name '*.onnx' 2>/dev/null | head -n 1 || true)"
  if [[ -n "$found" ]]; then
    printf '%s\n' "$found"
    return 0
  fi
  return 1
}

setup_native_env() {
  local include_dir lib_dir model_path cc_bin cxx_bin env_file go_bin

  include_dir="$(find_header_dir)" || {
    printf 'missing onnxruntime header: put onnxruntime_c_api.h under third_party/onnxruntime/include/\n' >&2
    exit 1
  }
  lib_dir="$(find_lib_dir)" || {
    printf 'missing onnxruntime library: put libonnxruntime.so under third_party/onnxruntime/lib/\n' >&2
    exit 1
  }
  ensure_unversioned_so "$lib_dir"
  if [[ ! -f "$lib_dir/libonnxruntime.so" ]]; then
    printf 'missing library: %s/libonnxruntime.so\n' "$lib_dir" >&2
    exit 1
  fi

  model_path="$(find_model_path || true)"
  if [[ -z "$model_path" ]]; then
    if [[ "$MODE" == "env" ]]; then
      printf 'warning: no silero onnx model found under third_party/models/\n' >&2
    else
      printf 'missing model: put silero_vad.onnx under third_party/models/\n' >&2
      exit 1
    fi
  fi

  cc_bin="$(pick_compiler "${CC:-}" cc gcc clang)" || {
    printf 'no C compiler found, please install cc/gcc/clang\n' >&2
    exit 1
  }
  cxx_bin="$(pick_compiler "${CXX:-}" c++ g++ clang++)" || {
    printf 'no C++ compiler found, please install c++/g++/clang++\n' >&2
    exit 1
  }
  go_bin="$(pick_go)" || {
    printf 'no Go binary found, please install go or set GO_BIN\n' >&2
    exit 1
  }

  export CGO_ENABLED=1
  export CC="$cc_bin"
  export CXX="$cxx_bin"
  export GO_BIN="$go_bin"
  export ONNXRUNTIME_INCLUDE_DIR="$include_dir"
  export ONNXRUNTIME_LIB_DIR="$lib_dir"
  export CGO_CFLAGS="-I$include_dir${CGO_CFLAGS:+ }${CGO_CFLAGS:-}"
  export CGO_LDFLAGS="-L$lib_dir -lonnxruntime${CGO_LDFLAGS:+ }${CGO_LDFLAGS:-}"
  export LD_LIBRARY_PATH="$lib_dir${LD_LIBRARY_PATH:+:}${LD_LIBRARY_PATH:-}"
  if [[ -n "$model_path" ]]; then
    export GOPBX_VAD_NATIVE_MODEL_PATH="$model_path"
  fi

  env_file="${ENV_FILE:-$ROOT_DIR/configs/gopbx.env.example}"
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi
}

print_env() {
  printf 'native VAD env loaded\n'
  printf '  CC=%s\n' "$CC"
  printf '  CXX=%s\n' "$CXX"
  printf '  GO_BIN=%s\n' "$GO_BIN"
  printf '  ONNXRUNTIME_INCLUDE_DIR=%s\n' "$ONNXRUNTIME_INCLUDE_DIR"
  printf '  ONNXRUNTIME_LIB_DIR=%s\n' "$ONNXRUNTIME_LIB_DIR"
  printf '  GOPBX_VAD_NATIVE_MODEL_PATH=%s\n' "${GOPBX_VAD_NATIVE_MODEL_PATH:-}"
}

setup_native_env

case "$MODE" in
  run)
    print_env
    cd "$ROOT_DIR"
    exec "$GO_BIN" run -tags silero_native ./cmd/gateway "$@"
    ;;
  check)
    print_env
    cd "$ROOT_DIR"
    exec "$GO_BIN" test -tags silero_native ./internal/adapter/outbound/vad "$@"
    ;;
  test)
    print_env
    cd "$ROOT_DIR"
    exec "$GO_BIN" test -tags silero_native ./... "$@"
    ;;
  env)
    print_env
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    printf 'unknown mode: %s\n\n' "$MODE" >&2
    usage >&2
    exit 1
    ;;
esac
