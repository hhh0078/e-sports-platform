// Package app 为公开静态资料提供一天Redis缓存；鉴权、交易及最终审核投影不进入缓存。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const publicCacheTTL = 24 * time.Hour

const publicCacheReadScript = `
if redis.call('EXISTS',KEYS[2]) == 0 then return {'',''} end
local generation=redis.call('GET',KEYS[1])
if not generation then return {'',''} end
if redis.call('HGET',KEYS[3],'generation') == generation then
 return {generation,redis.call('HGET',KEYS[3],'payload') or ''}
end
return {generation,''}`

const publicCacheFillScript = `
if redis.call('EXISTS',KEYS[2]) == 1 and redis.call('GET',KEYS[1]) == ARGV[1] then
 redis.call('HSET',KEYS[3],'generation',ARGV[1],'payload',ARGV[2])
 redis.call('EXPIRE',KEYS[3],ARGV[3])
 return 1
end
return 0`

// publicCacheKeys 对规范化参数取摘要，隔离环境、响应版本和公开视图，避免原始查询词出现在键中。
func (a *App) publicCacheKeys(key string) []string {
	hash := sha256.Sum256([]byte(key))
	base := a.redisKey("public-read:v1:")
	return []string{base + "generation", base + "ready", base + "data:" + hex.EncodeToString(hash[:])}
}

// publicCached 仅缓存调用方明确选择的公开基础数据，缓存故障回源；旧请求只能在原generation下回填。
func publicCached[T any](ctx context.Context, a *App, key string, load func() (T, error)) (T, error) {
	if !a.publicCacheEnabled.Load() || !a.publicCacheReady.Load() {
		return load()
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	values, err := a.Redis.Eval(cacheCtx, publicCacheReadScript, a.publicCacheKeys(key)).StringSlice()
	cancel()
	if err != nil {
		a.publicCacheReady.Store(false)
		return load()
	}
	generation := ""
	if len(values) == 2 {
		generation = values[0]
		if values[1] != "" {
			var cached T
			if json.Unmarshal([]byte(values[1]), &cached) == nil {
				return cached, nil
			}
		}
	}
	value, err := load()
	if err != nil || generation == "" {
		return value, err
	}
	raw, encodeErr := json.Marshal(value)
	if encodeErr == nil {
		fillCtx, finish := context.WithTimeout(ctx, 200*time.Millisecond)
		if e := a.Redis.Eval(fillCtx, publicCacheFillScript, a.publicCacheKeys(key), generation, raw, int(publicCacheTTL/time.Second)).Err(); e != nil {
			a.publicCacheReady.Store(false)
		}
		finish()
	}
	return value, nil
}

// publicFinancialSettings 仅供公开卡片装饰和规则展示，资金事务继续调用不带缓存的financialSettings。
func (a *App) publicFinancialSettings(ctx context.Context) (FinancialSettings, error) {
	return publicCached(ctx, a, "financial-rules", func() (FinancialSettings, error) {
		return financialSettings(ctx, a.DB)
	})
}
