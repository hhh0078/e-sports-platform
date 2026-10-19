// Package app 验证数据库启动错误不会把连接串中的认证资料写入日志。
package app

import (
	"strings"
	"testing"
)

// TestDatabaseConfigErrorHidesCredentials 覆盖 URL 密码和查询参数密码，配置错误只返回安全提示。
func TestDatabaseConfigErrorHidesCredentials(t *testing.T) {
	for _, dsn := range []string{
		"postgres://auditor:TEST_QUERY_SENTINEL@localhost/db?sslmode=invalid",
		"postgres://auditor@localhost/db?password=TEST_QUERY_SENTINEL&sslmode=invalid",
	} {
		_, err := New(t.Context(), Config{DatabaseURL: dsn})
		if err == nil || strings.Contains(err.Error(), "TEST_QUERY_SENTINEL") || !strings.Contains(err.Error(), "数据库连接配置无效") {
			t.Fatal("数据库配置错误未安全脱敏")
		}
	}
}
