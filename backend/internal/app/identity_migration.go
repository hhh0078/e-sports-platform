// Package app 为历史用户补充保守的未成年标记，并记录JSONB核验字段的中文数据库说明。
package app

import "context"

// migrateIdentityVerification 将缺失或无效的年龄标记默认置为未核验，已有明确布尔值及其他资料保持原值。
func migrateIdentityVerification(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `-- 身份核验迁移：历史用户默认按未成年处理，仅核验成功方可将is_minor改为false。
		UPDATE users SET data=jsonb_set(data,'{is_minor}','true'::jsonb,true)
		WHERE data->'is_minor' IS NULL OR jsonb_typeof(data->'is_minor') IS DISTINCT FROM 'boolean';
		COMMENT ON COLUMN users.data IS '用户资料JSONB；is_minor为是否未成年或尚未核验，默认true，仅成年人二要素核验成功后置false';
		COMMENT ON COLUMN identities.data IS '私密身份资料；identity_verified_at为成年核验时间，identity_source为核验来源，identity_adult为成年通过事实，identity_request_id为提供方请求摘要，identity_name_masked与identity_card_masked为脱敏信息，identity_consent_at为核验同意时间，不保存完整姓名身份证';`)
	return err
}
