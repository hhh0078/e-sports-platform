// Package app 根据私密部署配置一次性授予指定微信用户管理员权限，不提供公开注册管理员入口。
package app

import (
	"context"
	"crypto/subtle"
	"os"
)

// bootstrapWechatAdmin 只在官方 OpenID 精确匹配时执行一次，保留授予记录以防降权后重新提权。
func bootstrapWechatAdmin(ctx context.Context, q querier, u *User, openid string) error {
	expected := os.Getenv("BOOTSTRAP_ADMIN_OPENID")
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(openid)) != 1 {
		return nil
	}
	_, err := get[map[string]string](ctx, q, "settings", "bootstrap-admin", false)
	if err == nil {
		return nil
	}
	if !missing(err) {
		return err
	}
	u.Role = "admin"
	if err = put(ctx, q, "users", u.ID, *u); err != nil {
		return err
	}
	if err = put(ctx, q, "settings", "bootstrap-admin", map[string]string{"user_id": u.ID, "assigned_at": now()}); err != nil {
		return err
	}
	return audit(ctx, q, u.ID, "admin.bootstrap", u.ID, "通过私密部署配置一次性设置首位管理员")
}
