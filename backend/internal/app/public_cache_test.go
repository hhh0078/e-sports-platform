// Package app 使用独立真实PostgreSQL和Redis验证公共缓存命中、事务失效、旧回填和故障恢复，不连接生产。
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// enablePublicCacheTest 启用真实缓存但不启动计时worker，使竞态测试能精确控制提交和投递时序。
func enablePublicCacheTest(t *testing.T, a *App) {
	t.Helper()
	a.publicCacheEnabled.Store(true)
	if err := a.drainPublicCache(t.Context(), true); err != nil {
		t.Fatal(err)
	}
}

// cachePendingTest 读取隔离schema的待失效数量，用于验证回滚与崩溃恢复事实。
func cachePendingTest(t *testing.T, a *App) int {
	t.Helper()
	var count int
	if err := a.DB.QueryRow(t.Context(), "SELECT count(*) FROM public_cache_outbox").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestIntegrationPublicCacheHitTTLAndFailure 验证命中不再调用数据加载器、一天TTL、坏缓存和故障回源。
func TestIntegrationPublicCacheHitTTLAndFailure(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	loads := 0
	load := func() (string, error) { loads++; return "真实公开内容", nil }
	for i := 0; i < 2; i++ {
		if value, err := publicCached(t.Context(), api.App, "hit", load); err != nil || value != "真实公开内容" {
			t.Fatalf("读取失败：%q %v", value, err)
		}
	}
	if loads != 1 {
		t.Fatalf("命中仍调用加载器：%d", loads)
	}
	key := api.App.publicCacheKeys("hit")[2]
	ttl := api.App.Redis.TTL(t.Context(), key).Val()
	if ttl > publicCacheTTL || ttl < publicCacheTTL-time.Minute {
		t.Fatalf("TTL不是一天：%v", ttl)
	}
	api.App.Redis.HSet(t.Context(), key, "payload", "{broken")
	_, _ = publicCached(t.Context(), api.App, "hit", load)
	if loads != 2 {
		t.Fatal("损坏缓存未回源")
	}
	api.App.Redis.Del(t.Context(), api.App.publicCacheKeys("")[1])
	_, _ = publicCached(t.Context(), api.App, "hit", load)
	if loads != 3 {
		t.Fatal("就绪租约失效后仍命中旧缓存")
	}
	_, err := publicCached(t.Context(), api.App, "error", func() (string, error) { return "", errors.New("预期失败") })
	if err == nil || api.App.Redis.Exists(t.Context(), api.App.publicCacheKeys("error")[2]).Val() != 0 {
		t.Fatal("错误被缓存")
	}
}

// TestIntegrationPublicCacheRollbackAndDirectWrite 验证业务回滚不失效，直接SQL提交及审核putRaw仍可靠登记。
func TestIntegrationPublicCacheRollbackAndDirectWrite(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	ctx := t.Context()
	before := api.App.Redis.Get(ctx, api.App.publicCacheKeys("")[0]).Val()
	tx, err := api.App.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE games SET data=jsonb_set(data,'{name}','"回滚游戏"') WHERE id='delta'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if cachePendingTest(t, api.App) != 0 {
		t.Fatal("回滚留下失效事实")
	}
	if _, err = api.App.DB.Exec(ctx, `UPDATE games SET data=jsonb_set(data,'{name}','"直接修改"') WHERE id='delta'`); err != nil {
		t.Fatal(err)
	}
	if cachePendingTest(t, api.App) != 1 {
		t.Fatal("直接SQL未登记")
	}
	api.App.flushPublicCache(ctx)
	if cachePendingTest(t, api.App) != 0 || api.App.Redis.Get(ctx, api.App.publicCacheKeys("")[0]).Val() == before {
		t.Fatal("提交后未推进代际")
	}
	game, _ := get[Game](ctx, api.App.DB, "games", "delta", false)
	game.Subtitle = "审核失败，请查看消息"
	if err = putRaw(ctx, api.App.DB, "games", game.ID, game); err != nil {
		t.Fatal(err)
	}
	if cachePendingTest(t, api.App) != 1 {
		t.Fatal("审核putRaw绕过失效")
	}
}

// TestIntegrationPublicCacheOldFillRace 用真实数据库提交夹住慢加载，验证失效后的旧请求不能把旧值重新缓存。
func TestIntegrationPublicCacheOldFillRace(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	loaded, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := publicCached(t.Context(), api.App, "late-game", func() (Game, error) {
			game, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
			close(loaded)
			<-release
			return game, err
		})
		finished <- err
	}()
	<-loaded
	_, err := api.App.DB.Exec(t.Context(), `UPDATE games SET data=jsonb_set(data,'{name}','"新版本游戏"') WHERE id='delta'`)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	api.App.flushPublicCache(t.Context())
	close(release)
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
	if api.App.Redis.Exists(t.Context(), api.App.publicCacheKeys("late-game")[2]).Val() != 0 {
		t.Fatal("旧加载器重新填入缓存")
	}
	fresh, err := publicCached(t.Context(), api.App, "late-game", func() (Game, error) { return get[Game](t.Context(), api.App.DB, "games", "delta", false) })
	if err != nil || fresh.Name != "新版本游戏" {
		t.Fatal("未读到提交后的新版本", err)
	}
}

// TestIntegrationPublicCacheCommitOrder 验证先领取序号后提交的事务仍能再次失效，不按最大序号遗漏晚提交。
func TestIntegrationPublicCacheCommitOrder(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	ctx := t.Context()
	first, err := api.App.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(ctx)
	_, err = first.Exec(ctx, `UPDATE games SET data=jsonb_set(data,'{subtitle}','"较晚提交"') WHERE id='delta'`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = api.App.DB.Exec(ctx, `UPDATE games SET data=jsonb_set(data,'{subtitle}','"较早提交"') WHERE id='honor'`)
	if err != nil {
		t.Fatal(err)
	}
	api.App.flushPublicCache(ctx)
	generation := api.App.Redis.Get(ctx, api.App.publicCacheKeys("")[0]).Val()
	if err = first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if cachePendingTest(t, api.App) != 1 {
		t.Fatal("较小序号的晚提交事实被遗漏")
	}
	api.App.flushPublicCache(ctx)
	if api.App.Redis.Get(ctx, api.App.publicCacheKeys("")[0]).Val() == generation {
		t.Fatal("晚提交没有再次失效")
	}
}

// TestIntegrationPublicCacheDeliveryFailureRecovery 验证Redis失败会回滚独立投递事务，恢复后重投不丢失事实。
func TestIntegrationPublicCacheDeliveryFailureRecovery(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	_, err := api.App.DB.Exec(t.Context(), `UPDATE games SET data=jsonb_set(data,'{subtitle}','"待恢复内容"') WHERE id='delta'`)
	if err != nil {
		t.Fatal(err)
	}
	original := api.App.Redis
	broken := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	api.App.Redis = broken
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	err = api.App.drainPublicCache(ctx, false)
	cancel()
	api.App.Redis = original
	broken.Close()
	if err == nil || api.App.publicCacheReady.Load() || cachePendingTest(t, api.App) != 1 {
		t.Fatal("失败后未保留待恢复失效")
	}
	api.App.flushPublicCache(t.Context())
	if cachePendingTest(t, api.App) != 0 || !api.App.publicCacheReady.Load() {
		t.Fatal("重新连接后未恢复投递")
	}
	api.App.Redis.Del(t.Context(), api.App.publicCacheKeys("")[:2]...)
	api.App.flushPublicCache(t.Context())
	if api.App.Redis.Get(t.Context(), api.App.publicCacheKeys("")[0]).Val() == "" {
		t.Fatal("Redis数据丢失后未恢复代际")
	}
}

// TestIntegrationPublicCacheHTTPAndArticleIsolation 验证管理保存立即失效、公开分页键隔离、文章下架实时拒绝。
func TestIntegrationPublicCacheHTTPAndArticleIsolation(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	api.call("", "GET", "/games", nil, 200, nil)
	game, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if err != nil {
		t.Fatal(err)
	}
	game.Name = "缓存更新游戏"
	api.call("admin", "PATCH", "/admin/games/delta", game, 200, nil)
	var games []Game
	api.call("", "GET", "/games", nil, 200, &games)
	if games[0].Name != game.Name && !strings.Contains(fmt.Sprint(games), game.Name) {
		t.Fatal("管理保存后仍是旧游戏缓存")
	}
	var ids []string
	for index := 0; index < 3; index++ {
		var article Article
		api.call("admin", "POST", "/admin/articles", map[string]any{"title": fmt.Sprintf("缓存指南%d", index), "summary": "公开说明", "body_markdown": "# 使用指南\n请正常沟通。", "status": "published"}, 200, &article)
		ids = append(ids, article.ID)
	}
	var first, second articlePageData
	api.call("", "GET", "/articles?q=缓存指南&limit=1&offset=0", nil, 200, &first)
	api.call("", "GET", "/articles?q=缓存指南&limit=1&offset=1", nil, 200, &second)
	if first.Total != 3 || second.Total != 3 || len(first.Items) != 1 || len(second.Items) != 1 || first.Items[0].ID == second.Items[0].ID {
		t.Fatal("文章缓存串页")
	}
	api.call("", "GET", "/articles/"+ids[0], nil, 200, nil)
	_, err = api.App.DB.Exec(t.Context(), `UPDATE articles SET data=jsonb_set(data,'{status}','"hidden"') WHERE id=$1`, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	api.call("", "GET", "/articles/"+ids[0], nil, 404, nil)
	api.call("", "GET", "/admin/articles", nil, 401, nil)
	api.call("", "GET", "/services?mine=1", nil, 401, nil)
	api.call("", "GET", "/clubs?managed=1", nil, 401, nil)
}

// TestIntegrationPublicCacheWorkerRecovery 验证真实后台恢复无主动drain的提交，并在关闭后释放连接。
func TestIntegrationPublicCacheWorkerRecovery(t *testing.T) {
	api := newTestAPI(t)
	api.App.StartPublicCacheWorker(t.Context())
	_, err := api.App.DB.Exec(t.Context(), `UPDATE games SET data=jsonb_set(data,'{subtitle}','"后台恢复内容"') WHERE id='delta'`)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for cachePendingTest(t, api.App) != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if cachePendingTest(t, api.App) != 0 {
		t.Fatal("后台未恢复提交后的失效事实")
	}
	var loads atomic.Int32
	for index := 0; index < 2; index++ {
		_, err = publicCached(t.Context(), api.App, "worker-hit", func() (string, error) { loads.Add(1); return "恢复后缓存", nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	if loads.Load() != 1 {
		t.Fatal("后台恢复后未重新命中缓存")
	}
	api.App.stopPublicCacheWorker()
}

// TestIntegrationPublicCacheMediaRevocation 验证直接SQL撤销公开图片会失效HTML缓存，原图片立即返回审核占位。
func TestIntegrationPublicCacheMediaRevocation(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	image := api.uploadCommunityImage("admin", "article_image", "", communityPNG(t, 400, 900), 200)
	var article Article
	api.call("admin", "POST", "/admin/articles", map[string]any{"title": "图片审核缓存", "summary": "公开说明", "body_markdown": "正文\n\n![图片](" + image + ")", "status": "published"}, 200, &article)
	var before, after struct {
		BodyHTML string `json:"body_html"`
	}
	api.call("", "GET", "/articles/"+article.ID, nil, 200, &before)
	if !strings.Contains(before.BodyHTML, image) {
		t.Fatal("未缓存实际托管图")
	}
	_, err := api.App.DB.Exec(t.Context(), `UPDATE evidence SET data=jsonb_set(jsonb_set(data,'{moderation}','"rejected"'),'{data}','null') WHERE id=$1`, strings.TrimPrefix(image, "/api/media/"))
	if err != nil {
		t.Fatal(err)
	}
	if cachePendingTest(t, api.App) != 1 {
		t.Fatal("直接撤图未登记失效")
	}
	api.App.flushPublicCache(t.Context())
	api.call("", "GET", "/articles/"+article.ID, nil, 200, &after)
	if !strings.Contains(after.BodyHTML, rejectedImageURL) || strings.Contains(after.BodyHTML, image) {
		t.Fatal("撤图后复用旧HTML")
	}
}

// TestIntegrationPublicCacheKeepsAuthAndFundsLive 验证冻结即时生效，用户和钱包写入不进入静态缓存失效事务。
func TestIntegrationPublicCacheKeepsAuthAndFundsLive(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	api.call("", "GET", "/games", nil, 200, nil)
	_, err := api.App.DB.Exec(t.Context(), `UPDATE users SET data=jsonb_set(data,'{status}','"frozen"') WHERE id='admin'`)
	if err != nil {
		t.Fatal(err)
	}
	api.call("admin", "GET", "/admin/articles", nil, 403, nil)
	if err = putRaw(t.Context(), api.App.DB, "wallets", "player", Wallet{UserID: "player", AvailableCents: 12345}); err != nil {
		t.Fatal(err)
	}
	if cachePendingTest(t, api.App) != 0 {
		t.Fatal("普通用户或钱包事务增加了静态失效")
	}
	var wallet map[string]any
	api.call("player", "GET", "/wallet", nil, 200, &wallet)
	if wallet["available_cents"] != float64(12345) {
		t.Fatal("钱包未读取实时数据")
	}
}

// TestIntegrationPublicCacheAnotherDrainerClaim 验证其他实例已领取队列但尚未投递时，配置保存仍主动更换代际。
func TestIntegrationPublicCacheAnotherDrainerClaim(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	api.call("", "GET", "/games", nil, 200, nil)
	before := api.App.Redis.Get(t.Context(), api.App.publicCacheKeys("")[0]).Val()
	if _, err := api.App.DB.Exec(t.Context(), `UPDATE games SET data=jsonb_set(data,'{name}','"并发投递新游戏"') WHERE id='delta'`); err != nil {
		t.Fatal(err)
	}
	claim, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer claim.Rollback(t.Context())
	var id int64
	if err = claim.QueryRow(t.Context(), "SELECT id FROM public_cache_outbox LIMIT 1 FOR UPDATE").Scan(&id); err != nil {
		t.Fatal(err)
	}
	api.App.flushPublicCache(t.Context())
	if api.App.Redis.Get(t.Context(), api.App.publicCacheKeys("")[0]).Val() == before {
		t.Fatal("队列被其他实例领取导致主动失效被跳过")
	}
	var games []Game
	api.call("", "GET", "/games", nil, 200, &games)
	if !strings.Contains(fmt.Sprint(games), "并发投递新游戏") {
		t.Fatal("保存后仍读取旧缓存")
	}
}

// countPublicCacheDataKeys 只统计当前测试独占前缀的数据键，避免把会话和其它测试的缓存算入容量断言。
func countPublicCacheDataKeys(t *testing.T, a *App) int {
	t.Helper()
	count := 0
	iterator := a.Redis.Scan(t.Context(), 0, a.redisKey("public-read:v1:data:*"), 100).Iterator()
	for iterator.Next(t.Context()) {
		count++
	}
	if err := iterator.Err(); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestIntegrationPublicArticleCacheBounded 验证任意搜索分页不增长缓存，五个常用空页真实命中并在源修改后失效。
func TestIntegrationPublicArticleCacheBounded(t *testing.T) {
	api := newTestAPI(t)
	enablePublicCacheTest(t, api.App)
	for offset := 0; offset <= 80; offset += 20 {
		api.call("", "GET", fmt.Sprintf("/articles?limit=20&offset=%d", offset), nil, 200, nil)
	}
	if countPublicCacheDataKeys(t, api.App) != 5 {
		t.Fatal("五个常用空页未形成严格五个缓存键")
	}
	for index := 0; index < 40; index++ {
		api.call("", "GET", fmt.Sprintf("/articles?q=unique-miss-%d&limit=%d&offset=%d", index, index%100+1, index*1009), nil, 200, nil)
	}
	for _, path := range []string{
		"/articles?limit=1", "/articles?limit=10", "/articles?limit=50", "/articles?limit=100",
		"/articles?limit=20&offset=1", "/articles?limit=20&offset=21", "/articles?limit=20&offset=100", "/articles?limit=20&offset=1000000",
	} {
		api.call("", "GET", path, nil, 200, nil)
	}
	if countPublicCacheDataKeys(t, api.App) != 5 {
		t.Fatal("任意搜索、页大小或深偏移产生额外一天缓存")
	}
	if err := api.App.drainPublicCache(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	lock, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(t.Context())
	if _, err = lock.Exec(t.Context(), "LOCK TABLE articles IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	// 锁住文章表后仍能完成五页HTTP，证明命中不会重新执行文章count/list查询。
	client := api.Server.Client()
	previousTimeout := client.Timeout
	client.Timeout = 2 * time.Second
	defer func() { client.Timeout = previousTimeout }()
	for offset := 0; offset <= 80; offset += 20 {
		api.call("", "GET", fmt.Sprintf("/articles?limit=20&offset=%d", offset), nil, 200, nil)
	}
	if err = lock.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	article := Article{ArticleSummary: ArticleSummary{ID: "bounded-cache-live", Title: "容量修复后的文章", Status: "published", Revision: 1, CreatedAt: now()}, BodyMarkdown: "真实新增的公开正文"}
	if err = putRaw(t.Context(), api.App.DB, "articles", article.ID, article); err != nil {
		t.Fatal(err)
	}
	api.App.flushPublicCache(t.Context())
	var page articlePageData
	api.call("", "GET", "/articles?limit=20&offset=0", nil, 200, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != article.ID {
		t.Fatal("源修改后仍命中旧的空页")
	}
	for offset := 20; offset <= 80; offset += 20 {
		api.call("", "GET", fmt.Sprintf("/articles?limit=20&offset=%d", offset), nil, 200, nil)
	}
	if countPublicCacheDataKeys(t, api.App) != 5 {
		t.Fatal("失效重填导致五页缓存容量增长")
	}
}
