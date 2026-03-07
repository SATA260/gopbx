// 这个文件提供日志组件入口，负责给迁移阶段的日志统一附加结构化字段。

package observability

import (
	"log/slog"
	"os"
)

func NewLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func WithSession(logger *slog.Logger, sessionID string) *slog.Logger {
	if logger == nil {
		logger = NewLogger()
	}
	if sessionID == "" {
		return logger
	}
	return logger.With("session_id", sessionID)
}
