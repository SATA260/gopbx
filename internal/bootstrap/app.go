// 这个文件是应用装配入口，负责组装 Echo、会话管理器和各类业务处理器。

package bootstrap

import (
	"log/slog"

	"gopbx/internal/adapter/inbound/http"
	"gopbx/internal/adapter/outbound/storage"
	"gopbx/internal/app/callrecord"
	"gopbx/internal/app/session"
	"gopbx/internal/config"
	localmw "gopbx/internal/middleware"
	"gopbx/internal/observability"

	"github.com/labstack/echo/v4"
)

type App struct {
	Echo        *echo.Echo
	Config      *config.Config
	Sessions    *session.Manager
	CallRecords *callrecord.Manager
	Logger      *slog.Logger
	Metrics     *observability.Metrics
	Tracer      *observability.Tracer
}

func New(cfg *config.Config) *App {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(localmw.Recover(), localmw.RequestID(), localmw.AccessLog(), localmw.CORS())

	app := &App{
		Echo:        e,
		Config:      cfg,
		Sessions:    session.NewManager(),
		CallRecords: callrecord.NewManager(storage.NewCallRecordWriter(cfg)),
		Logger:      observability.NewLogger(),
		Metrics:     observability.NewMetrics(),
		Tracer:      observability.NewTracer(),
	}

	handlers := httpinbound.NewHandlers(cfg, app.Sessions, app.CallRecords, app.Logger, app.Metrics, app.Tracer)
	RegisterRoutes(e, handlers)

	return app
}
