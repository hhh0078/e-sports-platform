// Package app 验证失败管理请求不会重置公开缓存代际，成功写入仍在响应前主动失效，避免匿名流量迫使全站回源。
package app

import "testing"

// TestIntegrationPublicCacheRejectedWritesKeepGeneration 使用真实Redis核对401、403、400均保留缓存，合法管理写立即换代际。
func TestIntegrationPublicCacheRejectedWritesKeepGeneration(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	api.call("", "GET", "/games", nil, 200, nil)
	generationKey := api.App.publicCacheKeys("")[0]
	before := api.App.Redis.Get(t.Context(), generationKey).Val()
	if before == "" {
		t.Fatal("未建立真实缓存代际")
	}
	for _, request := range []struct {
		user   string
		body   map[string]any
		status int
	}{
		{"", map[string]any{}, 401},
		{"player", map[string]any{}, 403},
		{"admin", map[string]any{"unexpected_field": "应被同步拒绝"}, 400},
	} {
		api.call(request.user, "PATCH", "/admin/settings", request.body, request.status, nil)
		if current := api.App.Redis.Get(t.Context(), generationKey).Val(); current != before {
			t.Fatalf("失败管理请求%d重置了缓存代际", request.status)
		}
		if cachePendingTest(t, api.App) != 0 {
			t.Fatal("失败请求留下不应存在的源修改失效事实")
		}
	}
	game, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if err != nil {
		t.Fatal(err)
	}
	game.Name = "合法管理写入后已更新"
	api.call("admin", "PATCH", "/admin/games/delta", game, 200, nil)
	if current := api.App.Redis.Get(t.Context(), generationKey).Val(); current == "" || current == before {
		t.Fatal("合法管理写入未在返回前失效")
	}
	var games []Game
	api.call("", "GET", "/games", nil, 200, &games)
	for _, item := range games {
		if item.ID == game.ID && item.Name == game.Name {
			return
		}
	}
	t.Fatal("成功管理写入后仍返回旧缓存")
}
