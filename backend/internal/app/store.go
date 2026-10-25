// Package app 使用 PostgreSQL 持久化领域数据，以事务和唯一约束保护交易一致性。
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

var tables = []string{"users", "articles", "announcement_receipts", "games", "services", "clubs", "members", "club_deputies", "orders", "messages", "reviews", "audit", "settings", "evidence", "identities", "wallets", "wallet_entries", "wallet_escrows", "wallet_topups", "wallet_withdrawals", "wallet_deposit_refunds", "wallet_requests", "inbox", "inbox_receipts", "fund_receipts", "fund_lots", "fund_movements", "fund_audits", "order_adjustments", "order_payments", "order_refunds", "shares", "moderation_tasks", "moderation_targets", "subscription_deliveries", "subscription_authorizations", "subscription_preferences", "subscription_receipts", "subscription_callbacks"}

// now 返回统一的 UTC 时间文本。
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// newID 返回不可预测的实体标识。
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// validTable 限制动态 SQL 表名为内部白名单。
func validTable(table string) string {
	for _, t := range tables {
		if t == table {
			return t
		}
	}
	panic("非法表名")
}

// get 读取领域记录，lock 仅能在事务中开启以获得行级互斥。
func get[T any](ctx context.Context, q querier, table, id string, lock bool) (T, error) {
	var out T
	var raw []byte
	sql := "SELECT data FROM " + validTable(table) + " WHERE id=$1"
	if lock {
		sql += " FOR UPDATE"
	}
	err := q.QueryRow(ctx, sql, id).Scan(&raw)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}

// put 在现有事务中持久化完整领域记录。
func put(ctx context.Context, q querier, table, id string, value any) error {
	if s := reviewWriteState(ctx); s != nil && reviewedTable(table) {
		if pool, ok := q.(*pgxpool.Pool); ok {
			return reviewPoolWrite(ctx, pool, table, id, value, s)
		}
		return putWithReview(ctx, q, table, id, value, s)
	}
	return putRaw(ctx, q, table, id, value)
}

// putRaw 仅由可信业务与审核事务持久化数据，不递归触发审核。
func putRaw(ctx context.Context, q querier, table, id string, value any) error {
	// 领域消息与业务数据同事务落库；连接池直写也先转换成事务，外部微信发送只由后台执行。
	if domainNoticeTable(table) {
		if pool, ok := q.(*pgxpool.Pool); ok {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx.Rollback(ctx)
			if err = putRaw(ctx, tx, table, id, value); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var previous []byte
	if domainNoticeTable(table) {
		err = q.QueryRow(ctx, "SELECT data FROM "+validTable(table)+" WHERE id=$1 FOR UPDATE", id).Scan(&previous)
		if missing(err) {
			// 缺行没有可锁对象，仅在新建竞争时使用事务锁并重读；已有行保持原锁序避免锁反转。
			if _, err = q.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "jijie-domain-create:"+table+":"+id); err != nil {
				return err
			}
			err = q.QueryRow(ctx, "SELECT data FROM "+validTable(table)+" WHERE id=$1 FOR UPDATE", id).Scan(&previous)
		}
		if err != nil && !missing(err) {
			return err
		}
	}
	statement := "INSERT INTO " + validTable(table) + " (id,data) VALUES ($1,$2) ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data"
	if table == "evidence" {
		statement = `INSERT INTO evidence(id,data) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET data=CASE WHEN evidence.data->>'moderation'='rejected' THEN evidence.data || (EXCLUDED.data-'data'-'moderation'-'sha256'-'owner_id'-'order_id'-'public') ELSE EXCLUDED.data END`
	}
	_, err = q.Exec(ctx, statement, id, raw)
	if err == nil && domainNoticeTable(table) {
		err = captureDomainNotices(ctx, q, table, id, previous, raw)
	}
	return err
}

// list 使用内部限定条件读取记录，参数独立绑定避免注入。
func list[T any](ctx context.Context, q querier, table, where string, args ...any) ([]T, error) {
	out := []T{}
	rows, err := q.Query(ctx, "SELECT data FROM "+validTable(table)+" "+where, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var item T
		if err = rows.Scan(&raw); err != nil {
			return out, err
		}
		if err = json.Unmarshal(raw, &item); err != nil {
			return out, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// migrate 建立独立数据库中的表、中文备注及交易唯一索引。
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173001)"); err != nil {
		return err
	}
	names := map[string]string{"articles": "平台Markdown文章与发布状态", "announcement_receipts": "用户按公告版本跨设备已读记录", "users": "平台用户与权限", "games": "游戏主题与服务分类", "services": "服务发布与审核", "clubs": "俱乐部组织与认证", "members": "俱乐部挂靠申请与成员", "club_deputies": "俱乐部副管理员授权与撤销记录", "orders": "交易订单与状态", "messages": "订单沟通与文字证据", "reviews": "订单完成评价", "audit": "管理员操作审计", "settings": "平台运营配置", "evidence": "订单原始图片证据与完整性摘要", "identities": "微信私密身份映射", "inbox": "用户未读系统通知", "inbox_receipts": "同日审核拒绝通知防重复摘要", "moderation_tasks": "图文后置审核持久任务与人工决定", "moderation_targets": "图文字段单调版本与审核展示状态"}
	for table, label := range map[string]string{"subscription_deliveries": "微信订阅业务提醒持久发件箱", "subscription_authorizations": "微信一次性模板候选与消费记录", "subscription_preferences": "用户订阅场景偏好", "subscription_receipts": "订阅事件与请求幂等摘要", "subscription_callbacks": "微信验签订阅回调摘要"} {
		names[table] = label
	}
	for table, label := range map[string]string{"fund_audits": "不可变提现来源审核快照", "fund_receipts": "已核验外部收款根凭证", "fund_lots": "逐笔资金来源批次及剩余容量", "fund_movements": "不可变资金分配与溯源凭证", "wallets": "分区钱包余额与冻结资金", "wallet_entries": "不可变资金变动账单", "wallet_escrows": "逐订单实付托管及原路退款分账", "wallet_topups": "余额与押金充值付款记录", "wallet_withdrawals": "服务收入提现冻结与官方转账结果", "wallet_deposit_refunds": "平台押金多笔原路退还记录", "wallet_requests": "同账号幂等号资金用途与载荷绑定记录"} {
		names[table] = label
	}
	for table, label := range map[string]string{"order_adjustments": "订单加减价协商与双方确认", "order_payments": "原单及补款不可变收款事实和退款预留", "order_refunds": "按原交易的退款执行分笔", "shares": "永久公开分享目标与分享人映射"} {
		names[table] = label
	}
	for _, table := range tables {
		sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id TEXT PRIMARY KEY, data JSONB NOT NULL); COMMENT ON TABLE %s IS '%s'; COMMENT ON COLUMN %s.id IS '领域记录唯一标识'; COMMENT ON COLUMN %s.data IS '经过服务端类型和边界验证的完整领域数据';", table, table, names[table], table, table)
		if _, err = tx.Exec(ctx, sql); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS orders_number_unique ON orders ((data->>'order_no')); CREATE UNIQUE INDEX IF NOT EXISTS orders_payment_unique ON orders ((data->>'payment_transaction_id')) WHERE COALESCE(data->>'payment_transaction_id','')<>''; CREATE UNIQUE INDEX IF NOT EXISTS orders_refund_unique ON orders ((data->>'refund_id')) WHERE COALESCE(data->>'refund_id','')<>''; CREATE UNIQUE INDEX IF NOT EXISTS reviews_order_unique ON reviews ((data->>'order_id')); CREATE INDEX IF NOT EXISTS orders_buyer_idx ON orders ((data->>'buyer_id')); CREATE INDEX IF NOT EXISTS orders_provider_idx ON orders ((data->>'provider_id')); CREATE INDEX IF NOT EXISTS orders_status_idx ON orders ((data->>'status')); CREATE INDEX IF NOT EXISTS orders_refund_status_idx ON orders ((data->>'refund_status')); CREATE INDEX IF NOT EXISTS services_game_idx ON services ((data->>'game_id'),(data->>'status')); CREATE INDEX IF NOT EXISTS members_club_idx ON members ((data->>'club_id')); CREATE INDEX IF NOT EXISTS messages_order_idx ON messages ((data->>'order_id'));`)
	if err != nil {
		return err
	}
	// 历史接单字段统一为默认开启，不改变冻结状态、手机号验证、角色或其它用户资料。
	if _, err = tx.Exec(ctx, `-- 迁移旧用户接单标记；仅更新兼容字段，账号冻结与手机号门槛保持不变。
		UPDATE users SET data=jsonb_set(data,'{provider_enabled}','true'::jsonb,true)
		WHERE data->'provider_enabled' IS DISTINCT FROM 'true'::jsonb`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE INDEX IF NOT EXISTS inbox_user_time_idx ON inbox ((data->>'user_id'),(data->>'created_at'),id);`); err != nil {
		return err
	}
	if err = migratePublicContent(ctx, tx); err != nil {
		return err
	}
	if err = migrateManagementPermissions(ctx, tx); err != nil {
		return err
	}
	if err = migrateSubscriptions(ctx, tx); err != nil {
		return err
	}
	if err = migrateModeration(ctx, tx); err != nil {
		return err
	}
	if err = migrateWallet(ctx, tx); err != nil {
		return err
	}
	if err = migrateSupportContactComments(ctx, tx); err != nil {
		return err
	}
	if err = migrateOrderAdjustments(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// missing 判断记录是否不存在。
func missing(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
