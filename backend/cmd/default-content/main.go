// Package main 显式初始化默认运营资料，使用容器原数据库和Redis配置，不启动API、不创建登录会话或资金记录。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"jijie/internal/app"
)

// run 仅在明确apply及根管理员ID时执行幂等初始化；默认不建表、不运行应用自动种子。
func run(args []string) error {
	if len(args) == 0 || args[0] != "apply" {
		return fmt.Errorf("用法：jijie-default-content apply --actor-id 已有平台根管理员ID")
	}
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	actor := flags.String("actor-id", "", "已授权执行的现有平台根管理员ID")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *actor == "" {
		return fmt.Errorf("必须明确提供 --actor-id，禁止未知参数")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg := app.LoadConfig()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("数据库配置无效，请检查容器配置")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return fmt.Errorf("数据库连接失败，请检查容器配置")
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	defer client.Close()
	service := &app.App{DB: pool, Redis: client, Config: cfg}
	result, err := service.ApplyDefaultContent(ctx, *actor)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

// main 失败使用非零退出码，基础设施异常不回显可能带凭据的内部连接错误。
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "默认运营内容初始化失败：请检查参数、根管理员权限、轮播容量或原编号冲突；详细信息由本地执行上下文核对")
		os.Exit(1)
	}
}
