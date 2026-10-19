// Package app 负责微信手机号服务端兑换、开发绑定和发布交易前的私密认证校验。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var phoneDigits = regexp.MustCompile(`^[0-9]{6,15}$`)
var countryDigits = regexp.MustCompile(`^[0-9]{1,4}$`)
var localMobile = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

// wechatAccessToken 缓存官方稳定接口凭证，凭证与 AppID/AppSecret 绑定且不对客户端暴露。
func (a *App) wechatAccessToken(ctx context.Context, refresh bool) (string, error) {
	if err := a.wechatConfigured(); err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(a.Config.WechatAppID + ":" + a.Config.WechatSecret))
	key := a.redisKey("wechat-access:" + hex.EncodeToString(hash[:16]))
	if !refresh {
		token, err := a.Redis.Get(ctx, key).Result()
		if err == nil {
			return token, nil
		}
		if err != redis.Nil {
			return "", fail(503, "手机号验证服务暂时不可用")
		}
	}
	var wx struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
	}
	input := map[string]any{"grant_type": "client_credential", "appid": a.Config.WechatAppID, "secret": a.Config.WechatSecret, "force_refresh": refresh}
	if err := a.wechatJSON(ctx, "POST", "https://api.weixin.qq.com/cgi-bin/stable_token", input, &wx); err != nil {
		return "", err
	}
	if wx.ErrCode != 0 || wx.AccessToken == "" || wx.ExpiresIn <= 60 {
		return "", fail(502, "微信手机号服务尚未授权或配置有误，请联系管理员")
	}
	ttl := time.Duration(wx.ExpiresIn-60) * time.Second
	if ttl > 2*time.Hour {
		ttl = 2 * time.Hour
	}
	if err := a.Redis.Set(ctx, key, wx.AccessToken, ttl).Err(); err != nil {
		return "", fail(503, "手机号验证服务暂时不可用")
	}
	return wx.AccessToken, nil
}

// bindWechatPhone 仅接受官方一次性授权 code，拒绝客户端自行指定号码或微信身份。
func (a *App) bindWechatPhone(w http.ResponseWriter, r *http.Request) (any, error) {
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
		return nil, fail(400, "请通过微信按钮授权手机号")
	}
	var wx struct {
		ErrCode   int `json:"errcode"`
		PhoneInfo struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
			CountryCode     string `json:"countryCode"`
			Watermark       struct {
				AppID     string `json:"appid"`
				Timestamp int64  `json:"timestamp"`
			} `json:"watermark"`
		} `json:"phone_info"`
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, e := a.wechatAccessToken(r.Context(), attempt > 0)
		if e != nil {
			return nil, e
		}
		wx.ErrCode = 0
		if err = a.wechatJSON(r.Context(), "POST", "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(token), map[string]string{"code": in.Code}, &wx); err != nil {
			return nil, err
		}
		if wx.ErrCode != 40001 && wx.ErrCode != 40014 && wx.ErrCode != 42001 {
			break
		}
	}
	if wx.ErrCode != 0 {
		return nil, fail(422, "微信手机号授权失败或小程序未开通该能力，请重新授权或联系管理员")
	}
	info := wx.PhoneInfo
	if info.Watermark.AppID != a.Config.WechatAppID || !phoneDigits.MatchString(info.PurePhoneNumber) || !countryDigits.MatchString(info.CountryCode) {
		return nil, fail(502, "微信手机号验证结果无效，未保存号码")
	}
	if info.Watermark.Timestamp <= 0 || time.Since(time.Unix(info.Watermark.Timestamp, 0)) > 10*time.Minute || time.Until(time.Unix(info.Watermark.Timestamp, 0)) > time.Minute {
		return nil, fail(422, "手机号授权已过期，请重新授权")
	}
	return a.storeVerifiedPhone(r.Context(), u.ID, info.PurePhoneNumber, info.CountryCode, "wechat")
}

// bindDevPhone 为本地验收提供显式测试入口，生产环境无此能力。
func (a *App) bindDevPhone(w http.ResponseWriter, r *http.Request) (any, error) {
	if !a.Config.DevMode {
		return nil, fail(404, "接口不存在")
	}
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		PhoneNumber string `json:"phone_number"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	in.PhoneNumber = strings.TrimSpace(in.PhoneNumber)
	if !localMobile.MatchString(in.PhoneNumber) {
		return nil, fail(400, "测试手机号需为 11 位大陆手机号格式")
	}
	return a.storeVerifiedPhone(r.Context(), u.ID, in.PhoneNumber, "86", "dev")
}

// storeVerifiedPhone 持用户行锁合并号码，不覆盖 OpenID、同意版本和其他私密身份字段。
func (a *App) storeVerifiedPhone(ctx context.Context, id, phone, country, source string) (User, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	u, err := get[User](ctx, tx, "users", id, true)
	if err != nil {
		return u, err
	}
	if u.Status != "active" {
		return u, fail(403, "账号已被停用")
	}
	if err = mergeIdentity(ctx, tx, id, map[string]string{"phone_number": phone, "phone_country_code": country, "phone_verified_at": now(), "phone_source": source}); err != nil {
		return u, err
	}
	u.PhoneVerified = true
	if len(phone) < 8 {
		u.PhoneMasked = strings.Repeat("*", len(phone)-2) + phone[len(phone)-2:]
	} else {
		u.PhoneMasked = phone[:3] + strings.Repeat("*", len(phone)-7) + phone[len(phone)-4:]
	}
	if err = put(ctx, tx, "users", u.ID, u); err != nil {
		return u, err
	}
	return u, tx.Commit(ctx)
}

// requireVerifiedPhone 直接核对私密验证记录，不信任客户端或公开资料中的布尔值。
func (a *App) requireVerifiedPhone(r *http.Request, u User) error {
	identity, err := get[map[string]string](r.Context(), a.DB, "identities", u.ID, false)
	if err != nil && !missing(err) {
		return err
	}
	if identity["phone_verified_at"] == "" || !phoneDigits.MatchString(identity["phone_number"]) || (!a.Config.DevMode && identity["phone_source"] != "wechat") {
		return fail(428, "请先授权验证手机号，再发布服务、需求或下单")
	}
	return nil
}
