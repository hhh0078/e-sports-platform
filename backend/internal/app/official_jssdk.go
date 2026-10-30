// Package app 为已登录的公众号 H5 页面签发受同源限制的微信 JS-SDK 配置，令牌和票据仅在服务端缓存。
package app

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/redis/go-redis/v9"
)

// officialJSSDKURL 仅允许配置回跳地址同源的完整 HTTPS 页面，签名保留原始编码且拒绝片段及控制字符。
func officialJSSDKURL(value, configured string) error {
	page, err := url.Parse(value)
	base, baseErr := url.Parse(configured)
	if err != nil || baseErr != nil || len(value) == 0 || len(value) > 4096 || page.Scheme != "https" || base.Scheme != "https" || page.User != nil || base.User != nil || page.Hostname() == "" || base.Hostname() == "" || page.Opaque != "" || strings.ContainsAny(value, "#\\") || strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 || strings.IndexFunc(page.Path, unicode.IsControl) >= 0 || strings.Contains(page.Path, "\\") {
		return fail(400, "微信签名地址必须为已配置公众号同源的无片段HTTPS页面")
	}
	pagePort, basePort := page.Port(), base.Port()
	if pagePort == "" {
		pagePort = "443"
	}
	if basePort == "" {
		basePort = "443"
	}
	if !strings.EqualFold(page.Hostname(), base.Hostname()) || pagePort != basePort {
		return fail(400, "微信签名地址不属于当前公众号站点")
	}
	return nil
}

// officialCredentialCacheKey 以公众号 AppID 和密钥摘要隔离服务端缓存，密钥轮换后不复用旧凭证。
func (a *App) officialCredentialCacheKey(kind string, c commerceSettings) string {
	hash := sha256.Sum256([]byte(c.OfficialSecret))
	return a.redisKey("official-" + kind + ":" + c.OfficialAppID + ":" + hex.EncodeToString(hash[:16]))
}

// officialAccessToken 获取稳定版公众号服务凭证，与网页授权用户令牌及小程序令牌完全分开缓存。
func (a *App) officialAccessToken(ctx context.Context, c commerceSettings, refresh bool) (string, error) {
	key := a.officialCredentialCacheKey("access", c)
	if !refresh {
		cached, err := a.Redis.Get(ctx, key).Result()
		if err == nil {
			return cached, nil
		}
		if err != redis.Nil {
			return "", fail(503, "公众号配置服务暂时不可用")
		}
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
	}
	input := map[string]any{"grant_type": "client_credential", "appid": c.OfficialAppID, "secret": c.OfficialSecret, "force_refresh": refresh}
	if err := a.wechatJSON(ctx, "POST", "https://api.weixin.qq.com/cgi-bin/stable_token", input, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 || result.AccessToken == "" || result.ExpiresIn <= 60 {
		return "", fail(502, "公众号接口凭据获取失败，请核对权限与服务器白名单")
	}
	ttl := time.Duration(result.ExpiresIn-60) * time.Second
	if ttl > 2*time.Hour {
		ttl = 2 * time.Hour
	}
	if err := a.Redis.Set(ctx, key, result.AccessToken, ttl).Err(); err != nil {
		return "", fail(503, "公众号凭据暂时无法保存")
	}
	return result.AccessToken, nil
}

// officialJSAPITicket 缓存公众号 JS-SDK 票据，官方明确令牌失效时仅刷新一次，不向浏览器返回票据。
func (a *App) officialJSAPITicket(ctx context.Context, c commerceSettings) (string, error) {
	key := a.officialCredentialCacheKey("jsapi-ticket", c)
	cached, err := a.Redis.Get(ctx, key).Result()
	if err == nil {
		return cached, nil
	}
	if err != redis.Nil {
		return "", fail(503, "公众号配置服务暂时不可用")
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := a.officialAccessToken(ctx, c, attempt > 0)
		if err != nil {
			return "", err
		}
		var result struct {
			Ticket    string `json:"ticket"`
			ExpiresIn int    `json:"expires_in"`
			ErrCode   int    `json:"errcode"`
		}
		query := url.Values{"access_token": {token}, "type": {"jsapi"}}
		if err = a.wechatJSON(ctx, "GET", "https://api.weixin.qq.com/cgi-bin/ticket/getticket?"+query.Encode(), nil, &result); err != nil {
			return "", err
		}
		if attempt == 0 && (result.ErrCode == 40001 || result.ErrCode == 40014 || result.ErrCode == 42001) {
			continue
		}
		if result.ErrCode != 0 || result.Ticket == "" || result.ExpiresIn <= 60 {
			return "", fail(502, "公众号JS-SDK权限尚未就绪，请联系管理员")
		}
		ttl := time.Duration(result.ExpiresIn-60) * time.Second
		if ttl > 2*time.Hour {
			ttl = 2 * time.Hour
		}
		if err = a.Redis.Set(ctx, key, result.Ticket, ttl).Err(); err != nil {
			return "", fail(503, "公众号配置暂时无法保存")
		}
		return result.Ticket, nil
	}
	return "", fail(502, "公众号凭据刷新失败，请稍后重试")
}

// officialJSSDKSignature 严格使用微信规定的 SHA1 参数顺序和页面原始无片段 URL 生成签名。
func officialJSSDKSignature(ticket, nonce string, stamp int64, pageURL string) string {
	value := "jsapi_ticket=" + ticket + "&noncestr=" + nonce + "&timestamp=" + strconv.FormatInt(stamp, 10) + "&url=" + pageURL
	hash := sha1.Sum([]byte(value))
	return hex.EncodeToString(hash[:])
}

// officialJSSDK 仅向登录用户签发本站页面所需配置，固定官方上游且不返回 token、ticket 或 AppSecret。
func (a *App) officialJSSDK(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.actor(r); err != nil {
		return nil, err
	}
	if err := a.loginRate(r); err != nil {
		return nil, err
	}
	var in struct {
		URL string `json:"url"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	c := a.commerceSettings()
	if c.OfficialAppID == "" || c.OfficialSecret == "" || !validNotifyURL(c.OfficialRedirectURL) {
		return nil, fail(503, "公众号JS-SDK尚未配置，请联系管理员")
	}
	if err := officialJSSDKURL(in.URL, c.OfficialRedirectURL); err != nil {
		return nil, err
	}
	ticket, err := a.officialJSAPITicket(r.Context(), c)
	if err != nil {
		return nil, err
	}
	nonce, stamp := newID(), time.Now().Unix()
	w.Header().Set("Cache-Control", "no-store")
	return map[string]any{"appId": c.OfficialAppID, "timestamp": stamp, "nonceStr": nonce, "signature": officialJSSDKSignature(ticket, nonce, stamp, in.URL)}, nil
}
