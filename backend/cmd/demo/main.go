// Package main 提供显式演示数据导入、统计和保护性清理命令，不启动服务或创建登录身份。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"jijie/internal/app"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// run 仅连接已初始化的 PostgreSQL，不读取或修改 Redis 会话和生产管理员身份。
func run(args []string) error {
	if len(args) != 1 || args[0] != "seed" && args[0] != "status" && args[0] != "clear" {
		return fmt.Errorf("用法：jijie-demo seed|status|clear；seed 幂等导入，status 查看数量，clear 仅清理未关联真实业务的演示记录")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, app.LoadConfig().DatabaseURL)
	if err != nil {
		return fmt.Errorf("数据库连接配置无效，请检查 DATABASE_URL 的格式与连接选项")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return fmt.Errorf("数据库连接失败，请检查地址、认证与数据库状态")
	}
	service := &app.App{DB: pool}
	var counts map[string]int
	switch args[0] {
	case "seed":
		counts, err = service.SeedShowcase(ctx)
	case "clear":
		counts, err = service.ClearShowcase(ctx)
	default:
		counts, err = service.ShowcaseStatus(ctx)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"operation": args[0], "counts": counts, "notice": "演示数据为虚构展示，无真实资金、微信身份或手机号；生成数量为本次新增，status 可查看总数。"})
}

// main 输出简明命令结果，失败时返回非零状态供部署脚本检测。
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
