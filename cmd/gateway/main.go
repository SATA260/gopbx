// 这个文件是服务启动入口，负责加载配置、启动网关并处理优雅停机。

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gopbx/internal/bootstrap"
	"gopbx/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app := bootstrap.New(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeoutDuration())
		defer cancel()
		if err := app.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
