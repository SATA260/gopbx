// 这个文件提供日志组件入口，后续统一承接服务日志输出。

package observability

import (
	"log/slog"
	"os"
)

func NewLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
