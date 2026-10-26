// Package app 以事务内持久失效队列和事务外Redis generation刷新恢复公开静态缓存，避免旧回填与丢失失效。
package app

import (
	"context"
	"log"
	"net/http"
	"time"
)

// migratePublicCache 在相关源表写入同事务追加失效事实，不修改权限、订单或资金表。
func migratePublicCache(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `-- 公开静态缓存失效队列：追加记录避免全局版本行锁，回滚自动撤销失效事实。
CREATE TABLE IF NOT EXISTS public_cache_outbox(id BIGSERIAL PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
COMMENT ON TABLE public_cache_outbox IS '公开静态缓存待投递失效事实，提交后刷新Redis代际，失败及崩溃保留重试';
COMMENT ON COLUMN public_cache_outbox.id IS '失效事件唯一序号，仅用于领取，不作为事务提交先后顺序';
COMMENT ON COLUMN public_cache_outbox.created_at IS '失效事实登记时间';
CREATE OR REPLACE FUNCTION enqueue_public_cache_invalidation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE relevant boolean := true;
BEGIN
 IF TG_TABLE_NAME='settings' THEN
  IF TG_OP='DELETE' THEN relevant := OLD.id IN ('platform','agreements','financial','ai-private','public-content-v1');
  ELSE relevant := NEW.id IN ('platform','agreements','financial','ai-private','public-content-v1'); END IF;
 ELSIF TG_TABLE_NAME='evidence' THEN
  IF TG_OP='INSERT' THEN relevant := false;
  ELSIF TG_OP='DELETE' THEN relevant := COALESCE(OLD.data->>'public','false')='true';
  ELSE relevant := COALESCE(OLD.data->>'public','false')='true' AND (OLD.data->'moderation' IS DISTINCT FROM NEW.data->'moderation' OR OLD.data->'data' IS DISTINCT FROM NEW.data->'data'); END IF;
 END IF;
 IF TG_OP='UPDATE' THEN relevant := relevant AND OLD.data IS DISTINCT FROM NEW.data; END IF;
 IF relevant THEN INSERT INTO public_cache_outbox DEFAULT VALUES; END IF;
 RETURN NULL;
END $$;
COMMENT ON FUNCTION enqueue_public_cache_invalidation() IS '仅登记公开静态源变化；包含直接SQL审核和媒体撤销，不访问Redis或资金';`)
	if err != nil {
		return err
	}
	for _, table := range []string{"games", "settings", "articles", "evidence"} {
		if _, err = q.Exec(ctx, "DROP TRIGGER IF EXISTS public_cache_invalidation ON "+table+"; CREATE TRIGGER public_cache_invalidation AFTER INSERT OR UPDATE OR DELETE ON "+table+" FOR EACH ROW EXECUTE FUNCTION enqueue_public_cache_invalidation(); COMMENT ON TRIGGER public_cache_invalidation ON "+table+" IS '公开静态源变更同事务登记缓存失效';"); err != nil {
			return err
		}
	}
	return nil
}

// drainPublicCache 在独立短事务领取失效事实；Redis成功后才删队列，崩溃后的重复失效只增加缓存未命中。
func (a *App) drainPublicCache(ctx context.Context, reset bool) error {
	a.publicCacheDrainMu.Lock()
	defer a.publicCacheDrainMu.Unlock()
	a.publicCacheReady.Store(false)
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var pending bool
	if err = tx.QueryRow(ctx, `WITH claimed AS (SELECT id FROM public_cache_outbox ORDER BY id LIMIT 512 FOR UPDATE SKIP LOCKED), removed AS (DELETE FROM public_cache_outbox WHERE id IN (SELECT id FROM claimed) RETURNING id) SELECT EXISTS(SELECT 1 FROM removed)`).Scan(&pending); err != nil {
		return err
	}
	keys := a.publicCacheKeys("")[:2]
	_, err = a.Redis.Eval(ctx, `
if ARGV[1]=='1' or redis.call('EXISTS',KEYS[1])==0 then redis.call('SET',KEYS[1],ARGV[2]) end
redis.call('SET',KEYS[2],'1','EX',5)
return 1`, keys, boolText(pending || reset), newID()).Result()
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	a.publicCacheReady.Store(true)
	return nil
}

// boolText 生成Redis脚本所需的明确布尔参数，避免客户端隐式编码差异。
func boolText(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// flushPublicCache 在已经提交的配置写入后尽力立即失效；失败不把已成功的保存伪装成失败。
func (a *App) flushPublicCache(ctx context.Context) {
	if !a.publicCacheEnabled.Load() {
		return
	}
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	// 即使另一实例已领取队列但尚未投递，也主动更换代际后再返回配置保存结果。
	if err := a.drainPublicCache(flushCtx, true); err != nil {
		log.Printf("公开缓存失效暂缓，已持久化待恢复，当前进程回源")
	}
}

// publicCacheWriteRequest 只为可能修改静态源的管理路由主动失效，不给交易和普通个人事务增加Redis请求。
func publicCacheWriteRequest(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return false
	}
	switch r.Pattern {
	case "POST /api/admin/games", "PATCH /api/admin/games/{id}", "PATCH /api/admin/settings", "PATCH /api/admin/agreements", "PATCH /api/admin/financial-settings", "PATCH /api/admin/ai-settings", "PATCH /api/admin/public-content", "POST /api/admin/articles", "PATCH /api/admin/articles/{id}", "POST /api/admin/moderation/{id}/decision":
		return true
	}
	return false
}

// StartPublicCacheWorker 启动前清理旧代际，每秒恢复已提交失效；短就绪租约阻止后台失联后继续命中一天旧缓存。
func (a *App) StartPublicCacheWorker(ctx context.Context) {
	a.publicCacheWorkerMu.Lock()
	defer a.publicCacheWorkerMu.Unlock()
	if a.publicCacheCancel != nil {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	a.publicCacheCancel = cancel
	a.publicCacheEnabled.Store(true)
	initial, done := context.WithTimeout(workerCtx, 2*time.Second)
	if err := a.drainPublicCache(initial, true); err != nil {
		log.Printf("公开缓存初始化暂缓，读取回源数据库")
	}
	done()
	a.publicCacheWG.Add(1)
	go func() {
		defer a.publicCacheWG.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				attempt, finish := context.WithTimeout(workerCtx, 2*time.Second)
				_ = a.drainPublicCache(attempt, false)
				finish()
			}
		}
	}()
}

// stopPublicCacheWorker 等待独立失效事务结束后再释放数据库连接。
func (a *App) stopPublicCacheWorker() {
	a.publicCacheWorkerMu.Lock()
	cancel := a.publicCacheCancel
	a.publicCacheWorkerMu.Unlock()
	if cancel != nil {
		cancel()
		a.publicCacheWG.Wait()
	}
	a.publicCacheEnabled.Store(false)
	a.publicCacheReady.Store(false)
}
