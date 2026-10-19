// Package app 在事务内清理演示数据，发现真实业务引用时完整拒绝，保留真实交易与证据。
package app

import (
	"context"
	"fmt"
)

// showcaseTargets 在已锁定的表中收集双重标记的演示记录，不以模糊前缀批量删除。
func showcaseTargets(ctx context.Context, q querier) (map[string][]string, error) {
	targets := map[string][]string{}
	for _, table := range showcaseTables {
		rows, err := q.Query(ctx, "SELECT id FROM "+validTable(table)+" WHERE left(id,5)='demo_' AND data->>'is_demo'='true' AND data->>'demo_version'='1' ORDER BY id")
		if err != nil {
			return nil, err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		targets[table] = ids
	}
	return targets, nil
}

// protectShowcaseReferences 阻止删除被真实业务使用或已混入真实参与者、支付标识的演示记录。
func protectShowcaseReferences(ctx context.Context, q querier, targets map[string][]string) error {
	references := []struct{ table, field, target string }{
		{"clubs", "owner_id", "users"}, {"members", "user_id", "users"}, {"members", "club_id", "clubs"},
		{"services", "provider_id", "users"}, {"services", "club_id", "clubs"},
		{"orders", "buyer_id", "users"}, {"orders", "provider_id", "users"}, {"orders", "service_id", "services"}, {"orders", "club_id", "clubs"},
		{"messages", "order_id", "orders"}, {"messages", "sender_id", "users"}, {"reviews", "order_id", "orders"},
		{"evidence", "order_id", "orders"}, {"evidence", "owner_id", "users"}, {"evidence", "club_id", "clubs"},
	}
	for _, ref := range references {
		var id string
		err := q.QueryRow(ctx, "SELECT id FROM "+validTable(ref.table)+" WHERE NOT(id=ANY($1::text[])) AND data->>$2=ANY($3::text[]) LIMIT 1", targets[ref.table], ref.field, targets[ref.target]).Scan(&id)
		if err == nil {
			return fmt.Errorf("拒绝清理：%s/%s 已引用演示对象，请先人工核对真实业务", ref.table, id)
		}
		if !missing(err) {
			return err
		}
	}
	for _, ref := range []struct{ table, field, target string }{
		{"orders", "buyer_id", "users"}, {"orders", "provider_id", "users"}, {"orders", "service_id", "services"}, {"orders", "club_id", "clubs"},
		{"services", "provider_id", "users"}, {"services", "club_id", "clubs"}, {"clubs", "owner_id", "users"}, {"members", "user_id", "users"}, {"members", "club_id", "clubs"},
		{"messages", "order_id", "orders"}, {"messages", "sender_id", "users"}, {"reviews", "order_id", "orders"},
		{"evidence", "order_id", "orders"}, {"evidence", "owner_id", "users"}, {"evidence", "club_id", "clubs"},
	} {
		allowed := append([]string{}, targets[ref.target]...)
		if ref.table == "messages" && ref.field == "sender_id" {
			allowed = append(allowed, "demo_system")
		}
		if ref.table == "evidence" && (ref.field == "order_id" || ref.field == "club_id") {
			allowed = append(allowed, "")
		}
		var id string
		err := q.QueryRow(ctx, "SELECT id FROM "+validTable(ref.table)+" WHERE id=ANY($1::text[]) AND NOT(COALESCE(data->>$2,'')=ANY($3::text[])) LIMIT 1", targets[ref.table], ref.field, allowed).Scan(&id)
		if err == nil {
			return fmt.Errorf("拒绝清理：演示记录 %s 已关联非演示对象，需人工核对", id)
		}
		if !missing(err) {
			return err
		}
	}
	var id string
	err := q.QueryRow(ctx, "SELECT id FROM orders WHERE id=ANY($1::text[]) AND (COALESCE(data->>'payment_transaction_id','')<>'' OR COALESCE(data->>'pay_open_id','')<>'' OR COALESCE(data->>'refund_id','')<>'') LIMIT 1", targets["orders"]).Scan(&id)
	if err == nil {
		return fmt.Errorf("拒绝清理：演示订单 %s 已含支付标识，需人工核对", id)
	}
	if !missing(err) {
		return err
	}
	err = q.QueryRow(ctx, "SELECT id FROM identities WHERE id=ANY($1::text[]) LIMIT 1", targets["users"]).Scan(&id)
	if err == nil {
		return fmt.Errorf("拒绝清理：演示账号 %s 已关联身份资料，需人工核对", id)
	}
	if !missing(err) {
		return err
	}
	paths := make([]string, len(targets["evidence"]))
	for i, mediaID := range targets["evidence"] {
		paths[i] = "/api/media/" + mediaID
	}
	for _, ref := range []struct{ table, field string }{{"users", "avatar"}, {"users", "profile_cover"}, {"clubs", "cover_url"}, {"services", "cover_url"}, {"games", "icon_url"}, {"games", "cover_url"}} {
		err = q.QueryRow(ctx, "SELECT id FROM "+validTable(ref.table)+" WHERE NOT(id=ANY(COALESCE($1::text[],'{}'::text[]))) AND data->>$2=ANY($3::text[]) LIMIT 1", targets[ref.table], ref.field, paths).Scan(&id)
		if err == nil {
			return fmt.Errorf("拒绝清理：%s/%s 已使用演示图片", ref.table, id)
		}
		if !missing(err) {
			return err
		}
	}
	err = q.QueryRow(ctx, "SELECT id FROM clubs WHERE NOT(id=ANY($1::text[])) AND EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(data->'detail_blocks')='array' THEN data->'detail_blocks' ELSE '[]'::jsonb END) block WHERE block->>'image_url'=ANY($2::text[])) LIMIT 1", targets["clubs"], paths).Scan(&id)
	if err == nil {
		return fmt.Errorf("拒绝清理：俱乐部 %s 的详情已使用演示图片", id)
	}
	if !missing(err) {
		return err
	}
	err = q.QueryRow(ctx, "SELECT id FROM games WHERE EXISTS(SELECT 1 FROM jsonb_array_elements_text(CASE WHEN jsonb_typeof(data->'covers')='array' THEN data->'covers' ELSE '[]'::jsonb END) cover WHERE cover=ANY($1::text[])) LIMIT 1", paths).Scan(&id)
	if err == nil {
		return fmt.Errorf("拒绝清理：游戏 %s 的封面库已使用演示图片", id)
	}
	if !missing(err) {
		return err
	}
	return nil
}

// ClearShowcase 仅清理本版本演示命名空间，真实数据及其关联对象一律保留。
func (a *App) ClearShowcase(ctx context.Context) (map[string]int, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173040)"); err != nil {
		return nil, err
	}
	// 阻止检查与删除之间出现新的交易或证据引用，读取接口仍可正常执行。
	if _, err = tx.Exec(ctx, "LOCK TABLE users,clubs,members,services,orders,messages,reviews,evidence,identities,games IN EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	targets, err := showcaseTargets(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err = protectShowcaseReferences(ctx, tx, targets); err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, table := range []string{"reviews", "messages", "orders", "services", "members", "clubs", "evidence", "users"} {
		result, e := tx.Exec(ctx, "DELETE FROM "+validTable(table)+" WHERE id=ANY($1::text[]) AND left(id,5)='demo_' AND data->>'is_demo'='true' AND data->>'demo_version'='1'", targets[table])
		if e != nil {
			return nil, e
		}
		counts[table] = int(result.RowsAffected())
	}
	return counts, tx.Commit(ctx)
}
