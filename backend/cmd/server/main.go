// Package main 启动开机电竞 API 服务并在退出时优雅释放资源。
package main

import (
	"context"
	"jijie/internal/app"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// main 加载配置、连接存储并监听本地 API。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx, app.LoadConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	a.StartPublicCacheWorker(ctx)
	a.StartModerationWorker(ctx)
	a.StartSubscriptionWorker(ctx)
	srv := &http.Server{Addr: a.Config.Addr, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	log.Printf("开机电竞 API 启动于 %s，开发模式=%v", a.Config.Addr, a.Config.DevMode)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
