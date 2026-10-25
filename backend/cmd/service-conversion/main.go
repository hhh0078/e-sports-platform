// Package main 提供演示服务承接的只读预览、显式执行与非扣款核验命令，不启动API或变更资金。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"jijie/internal/app"
)

// run 使用容器原有配置连接现有数据库，禁止隐式建表、种子初始化或会话创建。
func run(args []string) error {
	if len(args) == 0 || args[0] != "plan" && args[0] != "apply" && args[0] != "verify" {
		return fmt.Errorf("用法：service-conversion plan --owner-id ID --file 预览.json | apply --file 预览.json | verify")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	owner := flags.String("owner-id", "", "已授权承接的真实管理员ID")
	file := flags.String("file", "", "受限转换预览文件路径")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("存在未知参数")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	a := &app.App{DB: pool, Config: cfg}
	var result any
	switch args[0] {
	case "plan":
		if *owner == "" || *file == "" {
			return fmt.Errorf("预览需要 --owner-id 和 --file")
		}
		plan, e := a.PlanShowcaseServices(ctx, *owner)
		if e != nil {
			return e
		}
		out, e := os.OpenFile(*file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return fmt.Errorf("无法新建预览文件，拒绝覆盖既有文件")
		}
		err = json.NewEncoder(out).Encode(plan)
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		result = map[string]any{"operation": "plan", "count": len(plan.Items), "digest": plan.Digest, "file": *file, "business_records_changed": false}
	case "apply":
		if *file == "" || *owner != "" {
			return fmt.Errorf("执行仅接受 --file，以已审阅预览绑定接单人")
		}
		raw, e := os.ReadFile(*file)
		if e != nil {
			return fmt.Errorf("无法读取转换预览")
		}
		var plan app.ServiceConversionPlan
		if err = json.Unmarshal(raw, &plan); err != nil {
			return fmt.Errorf("转换预览格式不正确")
		}
		result, err = a.ApplyShowcaseServices(ctx, plan)
	case "verify":
		if *owner != "" || *file != "" {
			return fmt.Errorf("核验不接受修改参数")
		}
		result, err = a.VerifyShowcaseServices(ctx)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

// main 仅输出公开转换摘要，失败不回显数据库凭据或私密微信身份。
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
