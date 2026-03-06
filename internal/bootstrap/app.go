// 这个文件是应用装配入口，负责组装 Echo、会话管理器和各类业务处理器。

package bootstrap

import (
	"gopbx/internal/adapter/inbound/http"
	"gopbx/internal/app/session"
	"gopbx/internal/config"
	localmw "gopbx/internal/middleware"

	"github.com/labstack/echo/v4"
)

type App struct {
	Echo     *echo.Echo
	Config   *config.Config
	Sessions *session.Manager
}

func New(cfg *config.Config) *App {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(localmw.Recover(), localmw.RequestID(), localmw.AccessLog(), localmw.CORS())

	app := &App{
		Echo:     e,
		Config:   cfg,
		Sessions: session.NewManager(),
	}

	handlers := httpinbound.NewHandlers(cfg, app.Sessions)
	RegisterRoutes(e, handlers)

	return app
}
