// Package app 验证微信手机号的真实协议请求、权限失败、交易门槛及公开响应隐私。
package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntegrationPhoneGateAndPrivateIdentity 覆盖原始号码不公开、私密认证门槛、拒绝伪造与正式环境关闭开发绑定。
func TestIntegrationPhoneGateAndPrivateIdentity(t *testing.T) {
	api := newTestAPI(t)
	// 用户公开布尔值即使为真，也不能绕过不存在的私密认证记录。
	if err := put(t.Context(), api.App.DB, "identities", "worker", map[string]string{"openid": "preserve-private-openid", "user_agreement_version": "accepted-version"}); err != nil {
		t.Fatal(err)
	}
	api.call("worker", "POST", "/orders", map[string]any{}, 428, nil)
	api.call("worker", "POST", "/requests", map[string]any{}, 428, nil)
	api.call("worker", "POST", "/services", map[string]any{}, 428, nil)
	api.call("worker", "PATCH", "/me", map[string]any{"phone_verified": true}, 400, nil)
	api.call("worker", "POST", "/me/phone/wechat", map[string]string{"phone_number": "13900139000"}, 400, nil)
	api.call("worker", "POST", "/me/phone/dev", map[string]string{"phone_number": "invalid"}, 400, nil)
	var tokenCalls, phoneCalls atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("手机号接口仅应使用POST")
		}
		if r.URL.Path == "/cgi-bin/stable_token" {
			tokenCalls.Add(1)
			var input map[string]any
			json.NewDecoder(r.Body).Decode(&input)
			if input["appid"] != "wx_auth_test" || input["secret"] != "test-secret-never-public" || input["grant_type"] != "client_credential" {
				t.Error("稳定token请求字段错误")
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "private-access-token", "expires_in": 7200})
			return
		}
		if r.URL.Path != "/wxa/business/getuserphonenumber" || r.URL.Query().Get("access_token") != "private-access-token" {
			t.Error("手机号官方地址错误")
		}
		phoneCalls.Add(1)
		var input map[string]string
		json.NewDecoder(r.Body).Decode(&input)
		if input["code"] == "no-permission" {
			json.NewEncoder(w).Encode(map[string]any{"errcode": 48001})
			return
		}
		appid := "wx_auth_test"
		stamp := time.Now().Unix()
		if input["code"] == "foreign-app" {
			appid = "wx_other"
		}
		if input["code"] == "stale" {
			stamp -= 3600
		}
		json.NewEncoder(w).Encode(map[string]any{"phone_info": map[string]any{"phoneNumber": "+8613900139000", "purePhoneNumber": "13900139000", "countryCode": "86", "watermark": map[string]any{"appid": appid, "timestamp": stamp}}})
	})
	api.call("worker", "POST", "/me/phone/wechat", map[string]string{"code": "no-permission"}, 422, nil)
	api.call("worker", "POST", "/me/phone/wechat", map[string]string{"code": "foreign-app"}, 502, nil)
	api.call("worker", "POST", "/me/phone/wechat", map[string]string{"code": "stale"}, 422, nil)
	api.call("worker", "POST", "/services", map[string]any{}, 428, nil)
	var user User
	api.call("worker", "POST", "/me/phone/wechat", map[string]string{"code": "official-authorized-code"}, 200, &user)
	if !user.PhoneVerified || user.PhoneMasked != "139****9000" {
		t.Fatalf("手机号脱敏错误: %v %s", user.PhoneVerified, user.PhoneMasked)
	}
	identity, err := get[map[string]string](t.Context(), api.App.DB, "identities", "worker", false)
	if err != nil || identity["phone_number"] != "13900139000" || identity["phone_source"] != "wechat" || identity["openid"] != "preserve-private-openid" || identity["user_agreement_version"] != "accepted-version" {
		t.Fatal("号码验证覆盖其他私密身份或未记录来源")
	}
	if tokenCalls.Load() != 1 || phoneCalls.Load() != 4 {
		t.Fatal("官方token未缓存或手机号调用次数错误")
	}
	for _, entry := range []struct{ user, path string }{{"worker", "/me"}, {"admin", "/admin/users"}, {"", "/players/worker"}, {"", "/services"}} {
		status, raw, e := api.request(entry.user, "GET", entry.path, nil)
		if e != nil || status != 200 {
			t.Fatalf("隐私检查请求失败: %s %d %v", entry.path, status, e)
		}
		for _, secret := range []string{"13900139000", "preserve-private-openid", "private-access-token", "test-secret-never-public", "phone_number", "phone_country_code", "phone_verified_at"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("%s 泄露私密字段 %s", entry.path, secret)
			}
		}
	}
	api.App.Config.DevMode = false
	api.call("worker", "POST", "/me/phone/dev", map[string]string{"phone_number": "13800138000"}, 404, nil)
	// 开发模式绑定的号码不被当作正式微信号码，官方验证过的用户仍能提交需求。
	api.call("player", "POST", "/requests", map[string]any{}, 428, nil)
	api.call("worker", "POST", "/requests", map[string]any{"game_id": "delta", "title": "已认证玩家组队需求", "requirements": "一起完成任务", "amount_cents": 2500}, 200, nil)
}

// TestIntegrationPhoneRefreshAndPrivacyBoundary 验证令牌失效后仅重取一次，短号码始终脱敏且失败不能写认证状态。
func TestIntegrationPhoneRefreshAndPrivacyBoundary(t *testing.T) {
	api := newTestAPI(t)
	var tokenCalls, phoneCalls atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			n := tokenCalls.Add(1)
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			if (n == 2) != (in["force_refresh"] == true) {
				t.Error("force_refresh 与重试不符")
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "refreshed-token", "expires_in": 7200})
			return
		}
		if phoneCalls.Add(1) == 1 {
			json.NewEncoder(w).Encode(map[string]any{"errcode": 42001})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"phone_info": map[string]any{"purePhoneNumber": "123456", "countryCode": "65", "watermark": map[string]any{"appid": "wx_auth_test", "timestamp": time.Now().Unix()}}})
	})
	var user User
	api.call("worker", "POST", "/me/phone/wechat", map[string]string{"code": "fresh-code"}, 200, &user)
	if tokenCalls.Load() != 2 || phoneCalls.Load() != 2 || user.PhoneMasked != "****56" {
		t.Fatalf("刷新或短号脱敏错误: %d %d %s", tokenCalls.Load(), phoneCalls.Load(), user.PhoneMasked)
	}
}
