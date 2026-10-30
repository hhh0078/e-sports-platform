// Package app 隔离公众号与小程序身份，只用微信服务端核验的 UnionID 安全关联原有平台账号。
package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"os"
)

// validWechatTicket 核验服务器短票据绑定的渠道与当前 AppID，不信任客户端指定身份。
func (a *App) validWechatTicket(v wechatTicket) bool {
	if v.Channel == "official_account" {
		c := a.commerceSettings()
		return c.Mode == "official_account" && c.OfficialAppID != "" && v.AppID == c.OfficialAppID
	}
	return (v.Channel == "" || v.Channel == "mini_program") && v.AppID != "" && v.AppID == a.Config.WechatAppID
}

// resolveWechatUserID 串行核对官方身份映射与唯一 UnionID；已存在两个账号时拒绝自动合并资产。
// q 必须为当前身份事务，v 必须来自已经校验的服务端一次性票据。
func resolveWechatUserID(ctx context.Context, q querier, v wechatTicket) (string, error) {
	if _, err := q.Exec(ctx, "SELECT pg_advisory_xact_lock(8173034)"); err != nil {
		return "", err
	}
	rows, err := q.Query(ctx, `SELECT id FROM identities WHERE
		(data->>'appid'=$1 AND data->>'openid'=$2)
		OR (data->>'official_appid'=$1 AND data->>'official_openid'=$2)
		OR ($3<>'' AND data->>'wechat_unionid'=$3)`, v.AppID, v.OpenID, v.UnionID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	id := ""
	for rows.Next() {
		var candidate string
		if err = rows.Scan(&candidate); err != nil {
			return "", err
		}
		if id != "" && id != candidate {
			return "", fail(409, "微信身份关联存在冲突，请联系管理员核对，原账号资产未合并")
		}
		id = candidate
	}
	if err = rows.Err(); err != nil {
		return "", err
	}
	if id == "" {
		hash := sha256.Sum256([]byte(v.AppID + ":" + v.OpenID))
		id = "wx_" + hex.EncodeToString(hash[:16])
	}
	stored, err := get[map[string]string](ctx, q, "identities", id, false)
	if err != nil && !missing(err) {
		return "", err
	}
	if v.UnionID != "" && stored["wechat_unionid"] != "" && stored["wechat_unionid"] != v.UnionID {
		return "", fail(409, "微信统一身份发生变化，请联系管理员核对")
	}
	appField, openField := "appid", "openid"
	if v.Channel == "official_account" {
		appField, openField = "official_appid", "official_openid"
	}
	if (stored[appField] != "" && stored[appField] != v.AppID) || (stored[openField] != "" && stored[openField] != v.OpenID) {
		return "", fail(409, "该平台账号已关联其他微信身份，请联系管理员核对")
	}
	return id, nil
}

// paymentOpenID 按下单 AppID 读取对应私密 OpenID，避免公众号与小程序付款身份串用。
func (a *App) paymentOpenID(ctx context.Context, q querier, userID, appID string) (string, error) {
	v, err := get[map[string]string](ctx, q, "identities", userID, false)
	if err != nil {
		return "", fail(503, "请先通过对应微信渠道登录后付款")
	}
	if appID != "" && v["official_appid"] == appID && v["official_openid"] != "" {
		return v["official_openid"], nil
	}
	// 旧版小程序认证记录未保存 appid 时，仅允许部署中的原小程序 AppID。
	if appID != "" && appID == a.Config.WechatAppID && (v["appid"] == "" || v["appid"] == appID) && v["openid"] != "" {
		return v["openid"], nil
	}
	return "", fail(409, "当前账号尚未关联本支付渠道的微信身份，请重新登录")
}

// bootstrapOfficialAdmin 只为私密部署中同时指定的公众号 AppID 和官方 OpenID 一次性初始化管理员，降权后不重授。
func bootstrapOfficialAdmin(ctx context.Context, q querier, u *User, v wechatTicket) error {
	appID, openid := os.Getenv("BOOTSTRAP_ADMIN_OFFICIAL_APP_ID"), os.Getenv("BOOTSTRAP_ADMIN_OFFICIAL_OPENID")
	if v.Channel != "official_account" || appID == "" || openid == "" || subtle.ConstantTimeCompare([]byte(appID), []byte(v.AppID)) != 1 || subtle.ConstantTimeCompare([]byte(openid), []byte(v.OpenID)) != 1 {
		return nil
	}
	_, err := get[map[string]string](ctx, q, "settings", "bootstrap-admin-official", false)
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
	if err = put(ctx, q, "settings", "bootstrap-admin-official", map[string]string{"user_id": u.ID, "assigned_at": now()}); err != nil {
		return err
	}
	return audit(ctx, q, u.ID, "admin.bootstrap.official", u.ID, "通过私密公众号身份配置一次性设置管理员")
}
