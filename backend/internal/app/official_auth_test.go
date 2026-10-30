// Package app 验证公众号授权、防重放、跨微信身份隔离以及小程序支付密钥的账号归属。
package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// configureOfficialLogin 为独立测试实例设置非生产公众号配置，不保存或请求真实微信凭据。
func configureOfficialLogin(api *testAPI) {
	api.App.paymentConfigMu.Lock()
	api.App.commerceConfig = &commerceSettings{Mode: "official_account", FeeBearer: "platform", OfficialAppID: "wx1234567890abcdef", OfficialSecret: "official-test-secret-only", OfficialRedirectURL: "https://example.com/auth/callback"}
	api.App.paymentConfigMu.Unlock()
}

// officialStartForTest 通过真实 HTTP 入口发起当前协议已同意的授权，并解析随机 state。
func officialStartForTest(t *testing.T, api *testAPI, path string) (string, agreementSet) {
	t.Helper()
	v, err := readAgreements(t.Context(), api.App.DB)
	if err != nil {
		t.Fatal(err)
	}
	body := loginConsent("", v)
	delete(body, "ticket")
	body["return_path"] = path
	var result struct {
		URL     string `json:"url"`
		Expires int    `json:"expires_in"`
	}
	api.call("", "POST", "/auth/official/start", body, 200, &result)
	u, err := url.Parse(result.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "open.weixin.qq.com" || u.Query().Get("redirect_uri") != "https://example.com/auth/callback" || u.Query().Get("scope") != "snsapi_userinfo" || len(u.Query().Get("state")) != 64 || result.Expires != 300 {
		t.Fatal("公众号授权地址未绑定配置与随机state")
	}
	return u.Query().Get("state"), v
}

// TestOfficialReturnPathRejectsExternalRoutes 覆盖外部地址、路径穿越、登录循环和有效站内回跳。
func TestOfficialReturnPathRejectsExternalRoutes(t *testing.T) {
	for _, value := range []string{"https://evil.example/x", "//evil.example/x", "/pages/../secret", "/pages/%2e%2e/secret", "/pages/account/login?next=x", "/pages/main/index#https://evil.example", "/pages/main/\\evil", "/pages/main/index\r\nLocation:x"} {
		if _, err := officialReturnPath(value); err == nil {
			t.Fatalf("接受了不安全回跳 %q", value)
		}
	}
	if value, err := officialReturnPath("/pages/orders/detail?id=order-test"); err != nil || value != "/pages/orders/detail?id=order-test" {
		t.Fatal("合法业务回跳被拒绝")
	}
}

// TestIntegrationOfficialOAuthIdentityAndReplay 验证公众号与小程序通过唯一官方 UnionID 关联且不覆盖原身份和资产。
func TestIntegrationOfficialOAuthIdentityAndReplay(t *testing.T) {
	api := newTestAPI(t)
	configureOfficialLogin(api)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sns/jscode2session":
			json.NewEncoder(w).Encode(map[string]string{"openid": "mini-openid-private", "unionid": "union-private-shared", "session_key": "mini-session-private"})
		case "/sns/oauth2/access_token":
			if r.URL.Query().Get("appid") != "wx1234567890abcdef" || r.URL.Query().Get("secret") != "official-test-secret-only" {
				t.Error("公众号凭据错用小程序配置")
			}
			json.NewEncoder(w).Encode(map[string]string{"openid": "official-openid-private", "access_token": "official-token-private", "scope": "snsapi_userinfo"})
		case "/sns/userinfo":
			json.NewEncoder(w).Encode(map[string]string{"openid": "official-openid-private", "unionid": "union-private-shared"})
		default:
			t.Errorf("未预期微信接口 %s", r.URL.Path)
			w.WriteHeader(500)
		}
	})
	var prepared struct {
		Ticket     string       `json:"ticket"`
		Agreements agreementSet `json:"agreements"`
		ReturnPath string       `json:"return_path"`
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "mini-valid"}, 200, &prepared)
	var miniLogin struct {
		Token      string `json:"token"`
		User       User   `json:"user"`
		FirstLogin bool   `json:"first_login"`
	}
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, prepared.Agreements), 200, &miniLogin)
	if !miniLogin.FirstLogin {
		t.Fatal("初次登录未标记新注册")
	}
	miniLogin.User.Nickname = "已有平台资料"
	if err := put(t.Context(), api.App.DB, "users", miniLogin.User.ID, miniLogin.User); err != nil {
		t.Fatal(err)
	}
	api.Tokens["linked-mini"] = miniLogin.Token
	key, err := api.App.miniProgramSession(t.Context(), miniLogin.User.ID)
	if err != nil || key.SessionKey != "mini-session-private" {
		t.Fatal("小程序session_key未保存")
	}
	ttl := api.App.Redis.TTL(t.Context(), api.App.redisKey("wechat-session:"+miniLogin.User.ID)).Val()
	if ttl <= 0 || ttl > 2*time.Hour {
		t.Fatal("session_key过期时间错误")
	}
	state, agreements := officialStartForTest(t, api, "/pages/orders/detail?id=original-order")
	api.call("", "POST", "/auth/official/prepare", map[string]string{"code": "official-valid", "state": state}, 200, &prepared)
	if prepared.ReturnPath != "/pages/orders/detail?id=original-order" {
		t.Fatal("丢失站内回跳")
	}
	api.call("", "POST", "/auth/official/prepare", map[string]string{"code": "official-valid", "state": state}, 401, nil)
	status, raw, err := api.request("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, agreements))
	if err != nil || status != 200 {
		t.Fatalf("公众号登录失败 %d %s %v", status, raw, err)
	}
	var official struct {
		Data struct {
			User       User `json:"user"`
			FirstLogin bool `json:"first_login"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &official); err != nil {
		t.Fatal(err)
	}
	if official.Data.User.ID != miniLogin.User.ID || official.Data.User.Nickname != "已有平台资料" || official.Data.FirstLogin {
		t.Fatal("公众号登录新建或覆盖了已有平台账号")
	}
	stored, err := get[map[string]string](t.Context(), api.App.DB, "identities", miniLogin.User.ID, false)
	if err != nil || stored["openid"] != "mini-openid-private" || stored["official_openid"] != "official-openid-private" || stored["official_appid"] != "wx1234567890abcdef" {
		t.Fatal("两渠道OpenID未隔离保存")
	}
	for _, secret := range []string{"mini-openid-private", "official-openid-private", "union-private-shared", "mini-session-private", "official-token-private", "official-test-secret-only"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("登录响应泄露微信身份秘密")
		}
	}
	miniID, err := api.App.paymentOpenID(t.Context(), api.App.DB, miniLogin.User.ID, "wx_auth_test")
	if err != nil || miniID != "mini-openid-private" {
		t.Fatal("小程序付款身份读取错误")
	}
	officialID, err := api.App.paymentOpenID(t.Context(), api.App.DB, miniLogin.User.ID, "wx1234567890abcdef")
	if err != nil || officialID != "official-openid-private" {
		t.Fatal("公众号付款身份读取错误")
	}
	if _, err = api.App.paymentOpenID(t.Context(), api.App.DB, miniLogin.User.ID, "wx-another"); err == nil {
		t.Fatal("允许跨AppID付款")
	}
	state, _ = officialStartForTest(t, api, "/pages/main/index")
	api.App.Redis.ExpireAt(t.Context(), api.App.redisKey("official-state:"+state), time.Now().Add(-time.Second))
	api.call("", "POST", "/auth/official/prepare", map[string]string{"code": "official-valid", "state": state}, 401, nil)
}

// TestIntegrationWechatIdentityConflictAndRefresh 覆盖不同账号不自动合并、切号无法刷新原付款身份以及秘密不回显。
func TestIntegrationWechatIdentityConflictAndRefresh(t *testing.T) {
	api := newTestAPI(t)
	configureOfficialLogin(api)
	if err := put(t.Context(), api.App.DB, "identities", "player", map[string]string{"openid": "original-mini", "appid": "wx_auth_test", "wechat_unionid": "same-official-union"}); err != nil {
		t.Fatal(err)
	}
	if err := put(t.Context(), api.App.DB, "identities", "outsider", map[string]string{"official_openid": "already-other-account", "official_appid": "wx1234567890abcdef"}); err != nil {
		t.Fatal(err)
	}
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sns/oauth2/access_token" {
			json.NewEncoder(w).Encode(map[string]string{"openid": "already-other-account", "unionid": "same-official-union", "access_token": "private-official-access"})
			return
		}
		openid := "original-mini"
		if r.URL.Query().Get("js_code") == "switched-account" {
			openid = "other-mini"
		}
		json.NewEncoder(w).Encode(map[string]string{"openid": openid, "session_key": "rotated-private-session"})
	})
	state, v := officialStartForTest(t, api, "/pages/main/index")
	var prepared struct {
		Ticket string `json:"ticket"`
	}
	api.call("", "POST", "/auth/official/prepare", map[string]string{"code": "valid", "state": state}, 200, &prepared)
	api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, v), 409, nil)
	api.call("player", "POST", "/auth/wechat/session", map[string]string{"code": "switched-account"}, 409, nil)
	if api.App.Redis.Exists(t.Context(), api.App.redisKey("wechat-session:player")).Val() != 0 {
		t.Fatal("切号失败仍写入付款密钥")
	}
	status, raw, err := api.request("player", "POST", "/auth/wechat/session", map[string]string{"code": "same-account"})
	if err != nil || status != 200 || strings.Contains(string(raw), "rotated-private-session") {
		t.Fatal("刷新支付态失败或泄露密钥")
	}
	session, err := api.App.miniProgramSession(t.Context(), "player")
	if err != nil || session.OpenID != "original-mini" || session.SessionKey != "rotated-private-session" {
		t.Fatal("正确账号刷新失败")
	}
	api.App.Redis.ExpireAt(t.Context(), api.App.redisKey("wechat-session:player"), time.Now().Add(-time.Second))
	if _, err = api.App.miniProgramSession(t.Context(), "player"); err == nil {
		t.Fatal("过期session仍可支付")
	}
}

// TestIntegrationOfficialAdminBootstrap 仅精确匹配独立私密公众号身份时初始化管理员，跨渠道与降权后均不重复赋权。
func TestIntegrationOfficialAdminBootstrap(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_OPENID", "")
	t.Setenv("BOOTSTRAP_ADMIN_OFFICIAL_APP_ID", "wx1234567890abcdef")
	t.Setenv("BOOTSTRAP_ADMIN_OFFICIAL_OPENID", "private-official-owner")
	api := newTestAPI(t)
	configureOfficialLogin(api)
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sns/jscode2session" {
			json.NewEncoder(w).Encode(map[string]string{"openid": "private-official-owner"})
			return
		}
		openid := "other-official-user"
		if r.URL.Query().Get("code") == "owner" {
			openid = "private-official-owner"
		}
		if r.URL.Query().Get("code") == "case-mismatch" {
			openid = "Private-official-owner"
		}
		json.NewEncoder(w).Encode(map[string]string{"openid": openid, "access_token": "private-official-user-token"})
	})
	// login 走随机state与官方code兑换后提交协议，不接受客户端角色或OpenID。
	login := func(code string) User {
		t.Helper()
		state, v := officialStartForTest(t, api, "/pages/main/index")
		var prepared struct {
			Ticket string `json:"ticket"`
		}
		api.call("", "POST", "/auth/official/prepare", map[string]string{"code": code, "state": state}, 200, &prepared)
		var result struct {
			User  User   `json:"user"`
			Token string `json:"token"`
		}
		api.call("", "POST", "/auth/wechat", loginConsent(prepared.Ticket, v), 200, &result)
		api.Tokens[code] = result.Token
		return result.User
	}
	if login("other").Role != "user" || login("case-mismatch").Role != "user" {
		t.Fatal("未匹配公众号身份获得管理员")
	}
	var miniPrepared struct {
		Ticket     string       `json:"ticket"`
		Agreements agreementSet `json:"agreements"`
	}
	api.call("", "POST", "/auth/wechat/prepare", map[string]string{"code": "same-openid-mini"}, 200, &miniPrepared)
	var mini struct {
		User User `json:"user"`
	}
	api.call("", "POST", "/auth/wechat", loginConsent(miniPrepared.Ticket, miniPrepared.Agreements), 200, &mini)
	if mini.User.Role != "user" {
		t.Fatal("小程序相同OpenID错误使用公众号管理员配置")
	}
	owner := login("owner")
	if owner.Role != "admin" {
		t.Fatal("显式配置的公众号身份未初始化管理员")
	}
	api.call("owner", "GET", "/admin/overview", nil, 200, nil)
	marker, err := get[map[string]string](t.Context(), api.App.DB, "settings", "bootstrap-admin-official", false)
	if err != nil || marker["user_id"] != owner.ID {
		t.Fatal("公众号初始化缺少一次性标记")
	}
	owner.Role = "user"
	if err = put(t.Context(), api.App.DB, "users", owner.ID, owner); err != nil {
		t.Fatal(err)
	}
	if login("owner").Role != "user" {
		t.Fatal("已降权公众号账号被私密配置重复授予管理员")
	}
	api.call("owner", "GET", "/admin/overview", nil, 403, nil)
}

// TestIntegrationOfficialSMSConversionOwner 允许真实公众号与短信身份参与原管理员工具，不放宽管理员和真实性门槛。
func TestIntegrationOfficialSMSConversionOwner(t *testing.T) {
	api := newTestAPI(t)
	u, err := get[User](t.Context(), api.App.DB, "users", "admin", false)
	if err != nil {
		t.Fatal(err)
	}
	u.Bio = "真实平台管理员简介"
	if err = put(t.Context(), api.App.DB, "users", u.ID, u); err != nil {
		t.Fatal(err)
	}
	identity := map[string]string{"official_appid": "wx1234567890abcdef", "official_openid": "real-official-admin", "phone_number": "13900139000", "phone_verified_at": now(), "phone_source": "sms_tencent"}
	if err = put(t.Context(), api.App.DB, "identities", u.ID, identity); err != nil {
		t.Fatal(err)
	}
	if _, err = conversionOwner(t.Context(), api.App.DB, u.ID, false); err != nil {
		t.Fatal("真实公众号短信管理员被旧微信手机号限定阻断")
	}
	identity["phone_source"] = "dev"
	if err = put(t.Context(), api.App.DB, "identities", u.ID, identity); err != nil {
		t.Fatal(err)
	}
	if _, err = conversionOwner(t.Context(), api.App.DB, u.ID, false); err == nil {
		t.Fatal("管理员工具接受了模拟手机号来源")
	}
}
