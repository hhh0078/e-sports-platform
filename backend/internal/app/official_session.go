// Package app 将小程序 session_key 仅保存在短期服务端 Redis 中，并校验刷新时的微信身份归属。
package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// wechatSession 为虚拟支付提供已核验的小程序签名身份，不进入公开资料或客户端响应。
type wechatSession struct {
	AppID      string `json:"appid"`
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid,omitempty"`
}

// exchangeMiniProgramCode 仅请求固定官方接口，返回绑定原小程序的短期身份与签名密钥。
func (a *App) exchangeMiniProgramCode(ctx context.Context, code string) (wechatSession, error) {
	if err := a.wechatConfigured(); err != nil {
		return wechatSession{}, err
	}
	query := url.Values{"appid": {a.Config.WechatAppID}, "secret": {a.Config.WechatSecret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	var wx struct {
		OpenID     string `json:"openid"`
		UnionID    string `json:"unionid"`
		SessionKey string `json:"session_key"`
		ErrCode    int    `json:"errcode"`
	}
	if err := a.wechatJSON(ctx, "GET", "https://api.weixin.qq.com/sns/jscode2session?"+query.Encode(), nil, &wx); err != nil {
		return wechatSession{}, err
	}
	if wx.ErrCode != 0 || !length(wx.OpenID, 1, 128) || len(wx.UnionID) > 128 || len(wx.SessionKey) > 256 {
		return wechatSession{}, fail(401, "微信登录凭证已失效或小程序未获授权，请重新登录")
	}
	return wechatSession{AppID: a.Config.WechatAppID, OpenID: wx.OpenID, UnionID: wx.UnionID, SessionKey: wx.SessionKey}, nil
}

// storeWechatSession 以两小时 TTL 保存服务端签名密钥，拒绝缺项或不同小程序的身份。
func (a *App) storeWechatSession(ctx context.Context, userID string, v wechatSession) error {
	if userID == "" || v.AppID != a.Config.WechatAppID || v.OpenID == "" || v.SessionKey == "" {
		return fail(401, "微信支付登录态无效，请重新登录")
	}
	raw, _ := json.Marshal(v)
	if err := a.Redis.Set(ctx, a.redisKey("wechat-session:"+userID), raw, 2*time.Hour).Err(); err != nil {
		return fail(503, "微信支付登录态暂时无法保存")
	}
	return nil
}

// miniProgramSession 读取并重新核对当前账号的小程序签名身份，过期时要求客户端重新 wx.login。
func (a *App) miniProgramSession(ctx context.Context, userID string) (wechatSession, error) {
	return a.miniProgramSessionWith(ctx, a.DB, userID)
}

// miniProgramSessionWith 沿用调用方事务核对身份，避免持有资金锁时为身份验证另取数据库连接。
func (a *App) miniProgramSessionWith(ctx context.Context, q querier, userID string) (wechatSession, error) {
	raw, err := a.Redis.Get(ctx, a.redisKey("wechat-session:"+userID)).Result()
	if err != nil {
		return wechatSession{}, fail(401, "微信支付登录态已过期，请重新获取微信身份")
	}
	var v wechatSession
	if json.Unmarshal([]byte(raw), &v) != nil || v.AppID != a.Config.WechatAppID || v.SessionKey == "" {
		return wechatSession{}, fail(401, "微信支付登录态无效，请重新登录")
	}
	openid, err := a.paymentOpenID(ctx, q, userID, v.AppID)
	if err != nil || openid != v.OpenID {
		return wechatSession{}, fail(401, "微信支付登录态与当前账号不一致")
	}
	return v, nil
}

// refreshWechatSession 只允许当前平台账号刷新同一小程序 OpenID，阻止 wx.login 切号替换付款人。
func (a *App) refreshWechatSession(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if err = a.loginRate(r); err != nil {
		return nil, err
	}
	var in struct {
		Code string `json:"code"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !length(in.Code, 1, 256) {
		return nil, fail(400, "微信登录凭证无效")
	}
	v, err := a.exchangeMiniProgramCode(r.Context(), in.Code)
	if err != nil {
		return nil, err
	}
	openid, err := a.paymentOpenID(r.Context(), a.DB, u.ID, v.AppID)
	if err != nil || openid != v.OpenID {
		return nil, fail(409, "微信账号已切换，请退出平台后重新登录")
	}
	if err = a.storeWechatSession(r.Context(), u.ID, v); err != nil {
		return nil, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return map[string]int{"expires_in": 7200}, nil
}
