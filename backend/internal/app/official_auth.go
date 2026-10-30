// Package app 实现公众号网页授权的一次性 state、固定回跳与服务端身份票据兑换，秘密不返回浏览器。
package app

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// officialOAuthState 只保存服务端发起时的渠道与站内回跳，state 使用随机值且五分钟一次性消费。
type officialOAuthState struct {
	AppID      string `json:"appid"`
	ReturnPath string `json:"return_path"`
}

// officialReturnPath 限制回跳为站内业务页，拒绝协议、反斜杠、控制字符及登录循环。
func officialReturnPath(value string) (string, error) {
	if value == "" {
		return "/pages/main/index?tab=me", nil
	}
	u, err := url.Parse(value)
	if err != nil || len(value) > 1024 || !strings.HasPrefix(value, "/pages/") || u.IsAbs() || u.Host != "" || u.Fragment != "" || strings.ContainsAny(value, "\\\r\n\x00") || strings.Contains(u.Path, "..") || strings.HasPrefix(u.Path, "/pages/account/login") {
		return "", fail(400, "登录回跳页面无效")
	}
	return value, nil
}

// officialLoginStart 在主动同意当前协议后生成官方授权地址，不接受客户端提供外部回调地址。
func (a *App) officialLoginStart(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := a.loginRate(r); err != nil {
		return nil, err
	}
	c := a.commerceSettings()
	if c.Mode != "official_account" || c.OfficialAppID == "" || c.OfficialSecret == "" || !validNotifyURL(c.OfficialRedirectURL) {
		return nil, fail(503, "公众号登录尚未配置，请联系管理员")
	}
	var in struct {
		ReturnPath string `json:"return_path"`
		consentInput
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if err := validateConsent(r.Context(), a.DB, in.consentInput); err != nil {
		return nil, err
	}
	path, err := officialReturnPath(in.ReturnPath)
	if err != nil {
		return nil, err
	}
	state := newID() + newID()
	raw, _ := json.Marshal(officialOAuthState{AppID: c.OfficialAppID, ReturnPath: path})
	if err = a.Redis.Set(r.Context(), a.redisKey("official-state:"+state), raw, 5*time.Minute).Err(); err != nil {
		return nil, fail(503, "登录服务暂时不可用")
	}
	query := url.Values{"appid": {c.OfficialAppID}, "redirect_uri": {c.OfficialRedirectURL}, "response_type": {"code"}, "scope": {"snsapi_userinfo"}, "state": {state}}
	w.Header().Set("Cache-Control", "no-store")
	return map[string]any{"url": "https://open.weixin.qq.com/connect/oauth2/authorize?" + query.Encode() + "#wechat_redirect", "expires_in": 300}, nil
}

// officialLoginPrepare 原子消费 state 后兑换微信 code；仅产生短票据，不直接建立平台会话。
func (a *App) officialLoginPrepare(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := a.loginRate(r); err != nil {
		return nil, err
	}
	c := a.commerceSettings()
	if c.Mode != "official_account" || c.OfficialAppID == "" || c.OfficialSecret == "" {
		return nil, fail(503, "公众号登录尚未配置，请联系管理员")
	}
	var in struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !length(in.Code, 1, 256) || len(in.State) != 64 {
		return nil, fail(400, "公众号授权凭据无效")
	}
	if _, err := hex.DecodeString(in.State); err != nil {
		return nil, fail(400, "公众号授权凭据无效")
	}
	raw, err := a.Redis.GetDel(r.Context(), a.redisKey("official-state:"+in.State)).Result()
	if err == redis.Nil {
		return nil, fail(401, "公众号登录已过期或已使用，请重新发起登录")
	}
	if err != nil {
		return nil, fail(503, "登录服务暂时不可用")
	}
	var saved officialOAuthState
	if json.Unmarshal([]byte(raw), &saved) != nil || saved.AppID != c.OfficialAppID {
		return nil, fail(401, "公众号授权渠道已变化，请重新登录")
	}
	query := url.Values{"appid": {c.OfficialAppID}, "secret": {c.OfficialSecret}, "code": {in.Code}, "grant_type": {"authorization_code"}}
	var wx struct {
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		ErrCode     int    `json:"errcode"`
	}
	if err = a.wechatJSON(r.Context(), "GET", "https://api.weixin.qq.com/sns/oauth2/access_token?"+query.Encode(), nil, &wx); err != nil {
		return nil, err
	}
	if wx.ErrCode != 0 || !length(wx.OpenID, 1, 128) || wx.AccessToken == "" {
		return nil, fail(401, "公众号授权已失效，请重新登录")
	}
	if wx.UnionID == "" && strings.Contains(wx.Scope, "snsapi_userinfo") {
		var profile struct {
			OpenID  string `json:"openid"`
			UnionID string `json:"unionid"`
			ErrCode int    `json:"errcode"`
		}
		profileQuery := url.Values{"access_token": {wx.AccessToken}, "openid": {wx.OpenID}, "lang": {"zh_CN"}}
		if err = a.wechatJSON(r.Context(), "GET", "https://api.weixin.qq.com/sns/userinfo?"+profileQuery.Encode(), nil, &profile); err != nil {
			return nil, err
		}
		if profile.ErrCode != 0 || profile.OpenID != wx.OpenID {
			return nil, fail(401, "公众号身份校验失败，请重新登录")
		}
		wx.UnionID = profile.UnionID
	}
	if len(wx.UnionID) > 128 {
		return nil, fail(502, "公众号身份响应无效")
	}
	agreements, err := readAgreements(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	ticket := newID() + newID()
	rawTicket, _ := json.Marshal(wechatTicket{OpenID: wx.OpenID, AppID: c.OfficialAppID, UnionID: wx.UnionID, Channel: "official_account"})
	if err = a.Redis.Set(r.Context(), a.redisKey("wechat-ticket:"+ticket), rawTicket, 5*time.Minute).Err(); err != nil {
		return nil, fail(503, "登录服务暂时不可用")
	}
	w.Header().Set("Cache-Control", "no-store")
	return map[string]any{"ticket": ticket, "expires_in": 300, "agreements": agreements, "return_path": saved.ReturnPath}, nil
}
