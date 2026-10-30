// Package app 验证公众号 JS-SDK 的固定签名、同源授权边界、独立凭据缓存与官方令牌恢复。
package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestOfficialJSSDKSignatureAndOrigin 使用独立 SHA1 固定向量验证参数编码不变，并拒绝外部签名和隐蔽域名。
func TestOfficialJSSDKSignatureAndOrigin(t *testing.T) {
	if got := officialJSSDKSignature("private-test-ticket", "nonce-test-123", 1700000000, "https://example.com/app?b=2&a=%2F"); got != "79d27610b1d87c30df9a89ceed764a7f15473acd" {
		t.Fatal("JS-SDK签名未保留官方参数顺序或原始URL编码")
	}
	base := "https://example.com/callback"
	for _, value := range []string{"http://example.com/app", "https://evil.example/app", "https://example.com.evil.example/app", "https://example.com@evil.example/app", "https://user:pass@example.com/app", "https://example.com:8443/app", "https://example.com/app#", "https://example.com/app#hash", "//example.com/app", "https://example.com/app\n", "https://example.com/%0d%0aX", "https://example.com/%5cprivate"} {
		if err := officialJSSDKURL(value, base); err == nil {
			t.Fatalf("不安全JS-SDK签名地址未被拒绝 %q", value)
		}
	}
	for _, value := range []string{"https://example.com/app?x=%2F&y=2", "https://EXAMPLE.com:443/app", "https://example.com/?next=%2Fpages%2Forders"} {
		if err := officialJSSDKURL(value, base); err != nil {
			t.Fatalf("同源有效页面被拒绝 %q: %v", value, err)
		}
	}
}

// TestIntegrationOfficialJSSDKPrivateCache 核验公众号独立 AppID、签名地址、无秘密响应和两次调用复用官方票据。
func TestIntegrationOfficialJSSDKPrivateCache(t *testing.T) {
	api := newTestAPI(t)
	configureOfficialLogin(api)
	var tokens, tickets atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			tokens.Add(1)
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if r.Method != "POST" || body["appid"] != "wx1234567890abcdef" || body["secret"] != "official-test-secret-only" || body["force_refresh"] != false {
				t.Error("JS-SDK错用小程序凭据或强制刷新")
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "private-official-service-token", "expires_in": 7200})
		case "/cgi-bin/ticket/getticket":
			tickets.Add(1)
			if r.Method != "GET" || r.URL.Query().Get("type") != "jsapi" || r.URL.Query().Get("access_token") != "private-official-service-token" {
				t.Error("JS-SDK票据协议错误")
			}
			json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "ticket": "private-test-ticket", "expires_in": 7200})
		default:
			t.Errorf("JS-SDK请求未知官方地址 %s", r.URL.Path)
			w.WriteHeader(500)
		}
	})
	api.call("", "POST", "/auth/official/js-sdk", map[string]string{"url": "https://example.com/app"}, 401, nil)
	api.call("player", "POST", "/auth/official/js-sdk", map[string]string{"url": "https://evil.example/app"}, 400, nil)
	if tokens.Load() != 0 || tickets.Load() != 0 {
		t.Fatal("不合法域名签名仍请求微信")
	}
	pageURL := "https://example.com/app?b=2&a=%2F"
	var config struct {
		AppID     string `json:"appId"`
		Timestamp int64  `json:"timestamp"`
		Nonce     string `json:"nonceStr"`
		Signature string `json:"signature"`
	}
	api.call("player", "POST", "/auth/official/js-sdk", map[string]string{"url": pageURL}, 200, &config)
	if config.AppID != "wx1234567890abcdef" || config.Timestamp <= 0 || len(config.Nonce) != 32 || config.Signature != officialJSSDKSignature("private-test-ticket", config.Nonce, config.Timestamp, pageURL) {
		t.Fatal("公众号JSSDK配置或精确URL签名不正确")
	}
	firstNonce := config.Nonce
	status, raw, err := api.request("worker", "POST", "/auth/official/js-sdk", map[string]string{"url": pageURL})
	if err != nil || status != 200 {
		t.Fatal("缓存签名失败")
	}
	var second struct {
		Data struct {
			Nonce string `json:"nonceStr"`
		} `json:"data"`
	}
	json.Unmarshal(raw, &second)
	if second.Data.Nonce == firstNonce || tokens.Load() != 1 || tickets.Load() != 1 {
		t.Fatal("未复用凭据缓存或重复使用随机签名nonce")
	}
	for _, secret := range []string{"private-official-service-token", "private-test-ticket", "official-test-secret-only", "access_token", "jsapi_ticket"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("JS-SDK响应泄露服务端凭据")
		}
	}
}

// TestIntegrationOfficialJSSDKRefreshExpiredToken 只对官方明确过期错误刷新一次，避免使用网页授权令牌或无限重试。
func TestIntegrationOfficialJSSDKRefreshExpiredToken(t *testing.T) {
	api := newTestAPI(t)
	configureOfficialLogin(api)
	var tokens, tickets atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			count := tokens.Add(1)
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["force_refresh"] != (count == 2) {
				t.Error("公众号凭据刷新次数不符")
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "private-refreshed-token", "expires_in": 7200})
			return
		}
		if tickets.Add(1) == 1 {
			json.NewEncoder(w).Encode(map[string]any{"errcode": 42001})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ticket": "private-refreshed-ticket", "expires_in": 7200})
	})
	api.call("player", "POST", "/auth/official/js-sdk", map[string]string{"url": "https://example.com/app"}, 200, nil)
	if tokens.Load() != 2 || tickets.Load() != 2 {
		t.Fatal("官方过期token未执行有界恢复")
	}
}
