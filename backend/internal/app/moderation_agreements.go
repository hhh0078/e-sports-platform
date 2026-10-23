// Package app 仅升级精确旧默认图文审核条款，保留运营者配置、历史版本与已同意记录。
package app

import "context"

// migratePostReviewDefaultAgreements 在协议发布锁下归档确切旧默认，管理员自定义或已占版本均保持原样。
func migratePostReviewDefaultAgreements(ctx context.Context, q querier) error {
	const marker = "default-post-review-agreements-v1"
	if _, err := get[map[string]string](ctx, q, "settings", marker, false); err == nil {
		return nil
	} else if !missing(err) {
		return err
	}
	if _, err := q.Exec(ctx, "SELECT pg_advisory_xact_lock(8173020)"); err != nil {
		return err
	}
	var customized bool
	if err := q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM audit WHERE data->>'action'='agreements.save')").Scan(&customized); err != nil {
		return err
	}
	if customized {
		return seedInsert(ctx, q, "settings", marker, map[string]string{"description": "运营者已配置协议，保留当前文本并由运营者主动发布新规则", "applied_at": now()})
	}
	current, err := get[agreementSet](ctx, q, "settings", "agreements", true)
	if missing(err) {
		current = legacyBrandAgreements()
	} else if err != nil {
		return err
	}
	legacy, brand, next := legacyDefaultAgreements(), legacyBrandAgreements(), defaultAgreements()
	for _, pair := range []struct {
		kind                string
		current             *agreementDocument
		legacy, brand, next agreementDocument
	}{{"user", &current.UserAgreement, legacy.UserAgreement, brand.UserAgreement, next.UserAgreement}, {"privacy", &current.PrivacyPolicy, legacy.PrivacyPolicy, brand.PrivacyPolicy, next.PrivacyPolicy}} {
		if !sameAgreement(*pair.current, pair.legacy) && !sameAgreement(*pair.current, pair.brand) {
			continue
		}
		archived, e := get[agreementDocument](ctx, q, "settings", agreementHistoryID(pair.kind, pair.next.Version), false)
		if e != nil && !missing(e) {
			return e
		}
		if e == nil && !sameAgreement(archived, pair.next) {
			continue
		}
		if e = seedInsert(ctx, q, "settings", agreementHistoryID(pair.kind, pair.current.Version), *pair.current); e != nil {
			return e
		}
		if e = seedInsert(ctx, q, "settings", agreementHistoryID(pair.kind, pair.next.Version), pair.next); e != nil {
			return e
		}
		*pair.current = pair.next
	}
	if err = putRaw(ctx, q, "settings", "agreements", current); err != nil {
		return err
	}
	return seedInsert(ctx, q, "settings", marker, map[string]string{"description": "仅精确旧默认条款升级后置审核，原历史及自定义完整保留", "applied_at": now()})
}
