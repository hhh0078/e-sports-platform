// Package app 通过真实 HTTP 微信替身及独立 PG/Redis 验证协议同意、短期票据与身份私密性。
package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockWechat 将代码中固定的微信地址转发到真实本地 HTTP 替身，禁止意外请求其他域名。
func mockWechat(t *testing.T, api *testAPI, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	base, _ := url.Parse(server.URL)
	api.App.Config.WechatAppID = "wx_auth_test"
	api.App.Config.WechatSecret = "test-secret-never-public"
	api.App.HTTPClient = &http.Client{Timeout: 3 * time.Second, Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "api.weixin.qq.com" {
			t.Errorf("不允许的官方地址: %s", r.URL.Host)
		}
		clone := r.Clone(r.Context())
		target := *r.URL
		target.Host = base.Host
		target.Scheme = base.Scheme
		clone.URL = &target
		return http.DefaultTransport.RoundTrip(clone)
	})}
}

// loginConsent 返回明确勾选两份当前协议的登录请求。
func loginConsent(ticket string, v agreementSet) map[string]any {
	return map[string]any{"ticket": ticket, "agreed": true, "user_agreement_version": v.UserAgreement.Version, "privacy_policy_version": v.PrivacyPolicy.Version}
}

// TestIntegrationWechatTicketAndAgreementVersions 覆盖无注册静默识别、当前版本同意、单次票据、隐私和合并写入。
func TestIntegrationWechatTicketAndAgreementVersions(t *testing.T) {
	api := newTestAPI(t)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/jscode2session" || r.Method != "GET" || r.URL.Query().Get("appid") != "wx_auth_test" || r.URL.Query().Get("secret") != "test-secret-never-public" || r.URL.Query().Get("grant_type") != "authorization_code" {
			t.Error("code2session 请求与官方协议不符")
		}
		if r.URL.Query().Get("js_code") == "invalid" {
			json.NewEncoder(w).Encode(map[string]any{"errcode": 40029})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"openid": "private-openid-user", "session_key": "private-session-key"})
	})
	var before int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM users").Scan(&before)
	sessionsBefore, _ := api.App.Redis.Keys(t.Context(), api.App.redisKey("session:*")).Result()
	var prepared struct {
		Ticket     string       `json:"ticket"`
		ExpiresIn  int          `json:"expires_in"`
		Agreements agreementSet `json:"agreements"`
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "valid"}, 200, &prepared)
	var after int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM users").Scan(&after)
	sessionsAfter, _ := api.App.Redis.Keys(t.Context(), api.App.redisKey("session:*")).Result()
	if before != after || len(sessionsBefore) != len(sessionsAfter) || len(prepared.Ticket) != 64 || prepared.ExpiresIn != 300 {
		t.Fatal("静默准备不应创建账号或长期会话")
	}
	ttl := api.App.Redis.TTL(t.Context(), api.App.redisKey("wechat-ticket:"+prepared.Ticket)).Val()
	if ttl <= 0 || ttl > 5*time.Minute {
		t.Fatalf("票据过期时间无效: %s", ttl)
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "invalid"}, 401, nil)
	api.call("", "POST", "/auth/wechat", map[string]any{"openid": "attacker", "agreed": true}, 400, nil)
	refused := loginConsent(prepared.Ticket, prepared.Agreements)
	refused["agreed"] = false
	api.call("", "POST", "/auth/wechat", refused, 400, nil)
	updated := prepared.Agreements
	updated.UserAgreement.Version = "2026-10-19.1"
	updated.UserAgreement.Content += "\n更新交易规则。"
	api.call("worker", "PATCH", "/admin/agreements", updated, 403, nil)
	api.call("", "GET", "/admin/agreements", nil, 401, nil)
	api.call("admin", "PATCH", "/admin/agreements", updated, 200, &updated)
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements), 409, nil)
	var login struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, updated), 200, &login)
	api.Tokens["wechat"] = login.Token
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, updated), 401, nil)
	identity, err := get[map[string]string](t.Context(), api.App.DB, "identities", login.User.ID, false)
	if err != nil || identity["openid"] != "private-openid-user" || identity["user_agreement_version"] != updated.UserAgreement.Version || identity["consented_at"] == "" {
		t.Fatalf("同意记录未保存: %v", err)
	}
	api.call("wechat", "POST", "/me/phone/dev", map[string]string{"phone_number": "13800138000"}, 200, nil)
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "new-valid"}, 200, &prepared)
	code, raw, err := api.request("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, updated))
	if err != nil || code != 200 {
		t.Fatalf("再次登录失败: %d %s %v", code, raw, err)
	}
	for _, secret := range []string{"private-openid-user", "private-session-key", "test-secret-never-public", "13800138000"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("身份秘密泄露: %s", secret)
		}
	}
	identity, err = get[map[string]string](t.Context(), api.App.DB, "identities", login.User.ID, false)
	if err != nil || identity["phone_number"] != "13800138000" || identity["openid"] != "private-openid-user" {
		t.Fatal("再次登录覆盖了私密手机号或openid")
	}
	changedSame := updated
	changedSame.UserAgreement.Content += "不可覆盖的修改"
	api.call("admin", "PATCH", "/admin/agreements", changedSame, 409, nil)
	historyReuse := updated
	historyReuse.UserAgreement.Version = "2026-10-18.1"
	api.call("admin", "PATCH", "/admin/agreements", historyReuse, 409, nil)
	var actual agreementSet
	api.call("", "GET", "/agreements", nil, 200, &actual)
	if actual.UserAgreement.Version != updated.UserAgreement.Version {
		t.Fatal("失败写入改变了当前协议")
	}
	api.call("", "POST", "/auth/dev", map[string]any{"user_id": "player", "agreed": true, "user_agreement_version": "stale", "privacy_policy_version": "stale"}, 409, nil)
}

// TestIntegrationWechatTicketAtomicConsumption 验证同一身份票据不能并发签发两个会话，过期票据也被拒绝。
func TestIntegrationWechatTicketAtomicConsumption(t *testing.T) {
	api := newTestAPI(t)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"openid": "single-use-user"})
	})
	var prepared struct {
		Ticket     string       `json:"ticket"`
		Agreements agreementSet `json:"agreements"`
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "valid"}, 200, &prepared)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, err := api.request("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements))
			if err != nil {
				t.Error(err)
			}
			statuses <- code
		}()
	}
	wg.Wait()
	close(statuses)
	success, rejected := 0, 0
	for status := range statuses {
		if status == 200 {
			success++
		}
		if status == 401 {
			rejected++
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("并发重放结果错误: 成功%d 拒绝%d", success, rejected)
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "valid-again"}, 200, &prepared)
	api.App.Redis.ExpireAt(t.Context(), api.App.redisKey("wechat-ticket:"+prepared.Ticket), time.Now().Add(-time.Second))
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements), 401, nil)
}
