// Package app 以微信官方凭据换取短期身份票据，并在明确同意当前协议后建立登录会话。
package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type wechatTicket struct {
	OpenID     string `json:"openid"`
	AppID      string `json:"appid"`
	UnionID    string `json:"unionid,omitempty"`
	SessionKey string `json:"session_key,omitempty"`
	Channel    string `json:"channel,omitempty"`
}

// wechatConfigured 检查官方服务凭证，缺失时不降级为模拟微信身份。
func (a *App) wechatConfigured() error {
	if a.Config.WechatAppID == "" || a.Config.WechatSecret == "" {
		return fail(503, "微信登录尚未配置，请联系管理员")
	}
	return nil
}

// wechatJSON 调用固定官方地址并限制响应体，不向日志及客户端泄露上游地址或秘密。
func (a *App) wechatJSON(ctx context.Context, method, address string, input any, out any) error {
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(raw))
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return fail(502, "微信服务请求无效")
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.HTTPClient == nil {
		return fail(502, "微信服务暂时不可用，请稍后重试")
	}
	// 官方凭证只能发给固定目标，307/308等跳转不能重放含AppSecret或身份code的请求。
	client := *a.HTTPClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return fail(502, "微信服务暂时不可用，请稍后重试")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 || response.StatusCode != http.StatusOK || json.Unmarshal(raw, out) != nil {
		return fail(502, "微信服务响应异常，请稍后重试")
	}
	return nil
}

// wechatPrepare 只兑换五分钟一次性身份票据，不创建用户、不下发长期登录会话。
func (a *App) wechatPrepare(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := a.wechatConfigured(); err != nil {
		return nil, err
	}
	if err := a.loginRate(r); err != nil {
		return nil, err
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !length(in.Code, 1, 256) {
		return nil, fail(400, "微信登录凭证无效")
	}
	wx, err := a.exchangeMiniProgramCode(r.Context(), in.Code)
	if err != nil {
		return nil, err
	}
	agreements, err := readAgreements(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	ticket := newID() + newID()
	raw, _ := json.Marshal(wechatTicket{OpenID: wx.OpenID, AppID: a.Config.WechatAppID, UnionID: wx.UnionID, SessionKey: wx.SessionKey, Channel: "mini_program"})
	if err = a.Redis.Set(r.Context(), a.redisKey("wechat-ticket:"+ticket), raw, 5*time.Minute).Err(); err != nil {
		return nil, fail(503, "登录服务暂时不可用")
	}
	return map[string]any{"ticket": ticket, "expires_in": 300, "agreements": agreements}, nil
}

// wechatLogin 在条款校验通过后原子消费官方身份票据，并合并私密资料及同意记录。
func (a *App) wechatLogin(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := a.loginRate(r); err != nil {
		return nil, err
	}
	var in struct {
		Ticket string `json:"ticket"`
		consentInput
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if len(in.Ticket) != 64 {
		return nil, fail(401, "登录凭据无效，请重新获取微信身份")
	}
	if _, err := hex.DecodeString(in.Ticket); err != nil {
		return nil, fail(401, "登录凭据无效，请重新获取微信身份")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = validateConsent(r.Context(), tx, in.consentInput); err != nil {
		return nil, err
	}
	raw, err := a.Redis.GetDel(r.Context(), a.redisKey("wechat-ticket:"+in.Ticket)).Result()
	if err == redis.Nil {
		return nil, fail(401, "微信身份凭据已过期或已使用，请重新登录")
	}
	if err != nil {
		return nil, fail(503, "登录服务暂时不可用")
	}
	var identity wechatTicket
	if json.Unmarshal([]byte(raw), &identity) != nil || identity.OpenID == "" || !a.validWechatTicket(identity) {
		return nil, fail(401, "微信身份凭据无效")
	}
	id, err := resolveWechatUserID(r.Context(), tx, identity)
	if err != nil {
		return nil, err
	}
	firstLogin := false
	u, err := get[User](r.Context(), tx, "users", id, false)
	if missing(err) {
		avatar, avatarErr := randomDefaultAvatar()
		if avatarErr != nil {
			return nil, fail(503, "默认头像暂时无法分配，请稍后重试")
		}
		u = User{ID: id, Nickname: "新队友", Avatar: avatar, Role: "user", Status: "active", ProviderEnabled: true, IsMinor: true, CreatedAt: now()}
		userRaw, _ := json.Marshal(u)
		inserted, insertErr := tx.Exec(r.Context(), "INSERT INTO users(id,data) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", id, userRaw)
		err = insertErr
		if err == nil {
			firstLogin = inserted.RowsAffected() == 1
		}
	}
	if err != nil {
		return nil, err
	}
	u, err = get[User](r.Context(), tx, "users", id, true)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		return nil, fail(403, "账号已被停用")
	}
	fields := consentFields(in.consentInput)
	if identity.Channel == "official_account" {
		fields["official_openid"] = identity.OpenID
		fields["official_appid"] = identity.AppID
	} else {
		fields["openid"] = identity.OpenID
		fields["appid"] = identity.AppID
	}
	if identity.UnionID != "" {
		fields["wechat_unionid"] = identity.UnionID
	}
	if err = mergeIdentity(r.Context(), tx, id, fields); err != nil {
		return nil, err
	}
	if identity.Channel != "official_account" {
		if err = bootstrapWechatAdmin(r.Context(), tx, &u, identity.OpenID); err != nil {
			return nil, err
		}
	} else if err = bootstrapOfficialAdmin(r.Context(), tx, &u, identity); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	if identity.Channel != "official_account" && identity.SessionKey != "" {
		if err = a.storeWechatSession(r.Context(), id, wechatSession{AppID: identity.AppID, OpenID: identity.OpenID, SessionKey: identity.SessionKey}); err != nil {
			return nil, err
		}
	}
	session, err := a.session(r.Context(), u)
	if err == nil {
		session.(map[string]any)["first_login"] = firstLogin
	}
	return session, err
}

// registerIdentityRoutes 显式注册公开协议、受限身份与管理员协议接口。
func (a *App) registerIdentityRoutes(mux *http.ServeMux) {
	routes := map[string]func(http.ResponseWriter, *http.Request) (any, error){
		"GET /api/agreements": a.agreements, "GET /api/admin/agreements": a.adminAgreements, "PATCH /api/admin/agreements": a.saveAgreements,
		"POST /api/auth/wechat/prepare": a.wechatPrepare, "POST /api/me/phone/wechat": a.bindWechatPhone, "POST /api/me/phone/dev": a.bindDevPhone,
		"POST /api/auth/official/start": a.officialLoginStart, "POST /api/auth/official/prepare": a.officialLoginPrepare,
		"POST /api/auth/official/js-sdk": a.officialJSSDK,
		"POST /api/auth/wechat/session":  a.refreshWechatSession,
		"POST /api/me/phone/sms/send":    a.sendPhoneSMS, "POST /api/me/phone/sms/verify": a.verifyPhoneSMS,
	}
	for path, handler := range routes {
		mux.HandleFunc(path, a.wrap(handler))
	}
}
