// Package main 验证演示数据命令的错误输出不泄露数据库认证资料。
package main

import (
	"strings"
	"testing"
)

// TestDemoConfigErrorHidesCredentials 验证演示命令拒绝错误连接配置且不输出查询参数密码。
func TestDemoConfigErrorHidesCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://auditor@localhost/db?password=TEST_QUERY_SENTINEL&sslmode=invalid")
	err := run([]string{"status"})
	if err == nil || strings.Contains(err.Error(), "TEST_QUERY_SENTINEL") || !strings.Contains(err.Error(), "数据库连接配置无效") {
		t.Fatal("演示命令的数据库配置错误未安全脱敏")
	}
}
