// Package app 使用 Redis 原子窗口保护聊天与上传资源，避免单个账号无限灌入数据。
package app

import (
	"context"
	"strconv"
	"time"
)

// throttle 将计数与过期放在同一 Redis 脚本中，服务故障时明确拒绝写入。
func (a *App) throttle(ctx context.Context, scope, user string, limit int64) error {
	key := a.redisKey("limit:" + scope + ":" + user + ":" + strconv.FormatInt(time.Now().Unix()/60, 10))
	count, err := a.Redis.Eval(ctx, `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],120) end; return n`, []string{key}).Int64()
	if err != nil {
		return fail(503, "限流服务暂时不可用，请稍后重试")
	}
	if count > limit {
		return fail(429, "操作过于频繁，请稍后再试")
	}
	return nil
}
