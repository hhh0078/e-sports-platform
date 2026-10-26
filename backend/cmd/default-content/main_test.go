// Package main 验证默认内容命令必须显式apply和现有管理员ID，错误参数在连接数据库之前拒绝。
package main

import "testing"

// TestDefaultContentArguments 验证错误参数不会执行初始化或连接实际基础设施。
func TestDefaultContentArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"status"}, {"apply"}, {"apply", "--actor-id", "admin", "unexpected"}, {"apply", "--unknown"}, {"fill-game-captions"}, {"fill-game-captions", "--actor-id", "admin", "unexpected"}, {"fill-game-captions", "--unknown"}} {
		if err := run(args); err == nil {
			t.Fatal("错误参数被接受", args)
		}
	}
}
