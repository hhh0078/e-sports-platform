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

var tables = []string{"users", "games", "services", "clubs", "members", "orders", "messages", "reviews", "audit", "settings", "evidence", "identities"}

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
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, "INSERT INTO "+validTable(table)+" (id,data) VALUES ($1,$2) ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data", id, raw)
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
	names := map[string]string{"users": "平台用户与权限", "games": "游戏主题与服务分类", "services": "服务发布与审核", "clubs": "俱乐部组织与认证", "members": "俱乐部挂靠申请与成员", "orders": "交易订单与状态", "messages": "订单沟通与文字证据", "reviews": "订单完成评价", "audit": "管理员操作审计", "settings": "平台运营配置", "evidence": "订单原始图片证据与完整性摘要", "identities": "微信私密身份映射"}
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
	return tx.Commit(ctx)
}

// missing 判断记录是否不存在。
func missing(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
