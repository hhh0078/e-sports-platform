// Package app 一次性升级开机电竞默认品牌配置，保留运营自定义、旧协议及历史交易证据。
package app

import (
	"context"
)

// migrateDefaultBrand 仅迁移精确旧默认字段；调用方须持有种子事务锁，成功后记录一次性标记。
// devMode 只允许开发库更新固定体验管理员，不改真实用户、聊天或资金快照。
func migrateDefaultBrand(ctx context.Context, q querier, devMode bool) error {
	const marker = "default-brand-kaiji-v1"
	if _, err := get[map[string]any](ctx, q, "settings", marker, false); err == nil {
		return nil
	} else if !missing(err) {
		return err
	}
	if _, err := q.Exec(ctx, `UPDATE settings SET data=jsonb_set(data,'{site_name}',to_jsonb($1::text))
		WHERE id='platform' AND data->>'site_name'='集结'
		AND data->>'announcement'='找队友，也成为队友 · 下单前请与打手确认服务范围'
		AND NOT EXISTS(SELECT 1 FROM audit WHERE data->>'action'='settings.save' AND data->>'target_id'='platform')`, "开机电竞"); err != nil {
		return err
	}
	if devMode {
		if _, err := q.Exec(ctx, `UPDATE users SET data=jsonb_set(data,'{nickname}',to_jsonb($1::text))
			WHERE id='admin' AND data->>'nickname'='集结管理员' AND data->>'role'='admin'
			AND data->>'bio'='平台运营与纠纷处理'`, "开机电竞管理员"); err != nil {
			return err
		}
	}
	if _, err := q.Exec(ctx, `UPDATE settings SET data=jsonb_set(data,'{transfer_remark}',to_jsonb($1::text))
		WHERE id='wallet' AND data->>'transfer_remark'='集结服务收入提现'`, "开机电竞服务收入提现"); err != nil {
		return err
	}
	if err := migrateDefaultBrandAgreements(ctx, q); err != nil {
		return err
	}
	return seedInsert(ctx, q, "settings", marker, map[string]string{"description": "仅升级确切旧默认品牌及默认协议，保留管理员自定义和历史证据", "applied_at": now()})
}

// migrateDefaultBrandAgreements 在协议发布锁内归档旧默认并发布新版本，已定制条款逐份保留。
// 当前条款未持久化时，归档曾作为读取回退值的旧默认，确保旧同意版本仍能追溯。
func migrateDefaultBrandAgreements(ctx context.Context, q querier) error {
	if _, err := q.Exec(ctx, "SELECT pg_advisory_xact_lock(8173020)"); err != nil {
		return err
	}
	// 已由运营者正式保存的协议属于配置，不随默认品牌或审核流程自动改写。
	var operatorPublished bool
	if err := q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM audit WHERE data->>'action'='agreements.save')").Scan(&operatorPublished); err != nil {
		return err
	}
	if operatorPublished {
		return nil
	}
	legacy, next := legacyDefaultAgreements(), defaultAgreements()
	current, err := get[agreementSet](ctx, q, "settings", "agreements", true)
	if missing(err) {
		current = legacy
	} else if err != nil {
		return err
	}
	for _, pair := range []struct {
		kind    string
		current *agreementDocument
		legacy  agreementDocument
		next    agreementDocument
	}{{"user", &current.UserAgreement, legacy.UserAgreement, next.UserAgreement}, {"privacy", &current.PrivacyPolicy, legacy.PrivacyPolicy, next.PrivacyPolicy}} {
		if !sameAgreement(*pair.current, pair.legacy) {
			continue
		}
		conflict := false
		for _, doc := range []agreementDocument{*pair.current, pair.next} {
			history, err := get[agreementDocument](ctx, q, "settings", agreementHistoryID(pair.kind, doc.Version), false)
			if err != nil && !missing(err) {
				return err
			}
			if err == nil && !sameAgreement(history, doc) {
				conflict = true
			}
		}
		// 运营者已占用版本号时保留原公开文本，不能覆盖不可变历史或混淆同意证据。
		if conflict {
			continue
		}
		if err = seedInsert(ctx, q, "settings", agreementHistoryID(pair.kind, pair.current.Version), *pair.current); err != nil {
			return err
		}
		if err = seedInsert(ctx, q, "settings", agreementHistoryID(pair.kind, pair.next.Version), pair.next); err != nil {
			return err
		}
		*pair.current = pair.next
	}
	return put(ctx, q, "settings", "agreements", current)
}
