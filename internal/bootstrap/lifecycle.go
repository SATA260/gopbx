// 这个文件负责服务生命周期管理，封装启动与优雅关闭流程。

package bootstrap

import "context"

func (a *App) Start() error {
	return a.Echo.Start(a.Config.Server.Address)
}

func (a *App) Shutdown(ctx context.Context) error {
	return a.Echo.Shutdown(ctx)
}
