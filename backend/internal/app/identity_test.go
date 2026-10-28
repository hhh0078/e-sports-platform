// Package app 验证身份证年龄、提供方协议、秘密加密及用户交易门禁，所有证件均为本地合成夹具。
package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testIdentityCard 根据测试日期计算校验位，仅供离线替身测试，不向真实提供方发送。
func testIdentityCard(birthday string) string {
	prefix := "110101" + birthday + "001"
	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	sum := 0
	for i, weight := range weights {
		sum += int(prefix[i]-'0') * weight
	}
	return prefix + string("10X98765432"[sum%11])
}

// setTestAdultIdentity 显式构造隔离测试的成年私密事实，开发模式本身不跳过核验。
func setTestAdultIdentity(t *testing.T, api *testAPI, id string, adult bool) {
	t.Helper()
	u, err := get[User](t.Context(), api.App.DB, "users", id, false)
	if err != nil {
		t.Fatal(err)
	}
	u.IsMinor = !adult
	if err = put(t.Context(), api.App.DB, "users", id, u); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"identity_verified_at": "", "identity_adult": "", "identity_source": ""}
	if adult {
		fields = map[string]string{"identity_verified_at": now(), "identity_adult": "true", "identity_source": "xxapi"}
	}
	if err = mergeIdentity(t.Context(), api.App.DB, id, fields); err != nil {
		t.Fatal(err)
	}
}

// TestIdentityAdultAndLegacyDefaults 覆盖校验位、非法生日、上海满十八岁当天及旧用户默认拒绝。
func TestIdentityAdultAndLegacyDefaults(t *testing.T) {
	at := time.Date(2026, 10, 28, 0, 0, 0, 0, time.FixedZone("上海", 28800))
	for _, test := range []struct {
		date  string
		adult bool
		valid bool
	}{{"20081028", true, true}, {"20081029", false, true}, {"20081027", true, true}, {"20150229", false, false}, {"20080229", true, true}, {"20261029", false, false}, {"18991028", false, false}} {
		t.Run(test.date, func(t *testing.T) {
			adult, err := identityAdult(testIdentityCard(test.date), at.UTC())
			if (err == nil) != test.valid || adult != test.adult {
				t.Fatalf("年龄边界错误 adult=%v err=%v", adult, err)
			}
		})
	}
	for _, card := range []string{"", "11010120081028001X", "11010120081028001", "000000200810280011"} {
		if _, err := identityAdult(card, at); err == nil {
			t.Fatal("非法身份证被接受")
		}
	}
	for _, input := range []string{`{"id":"old"}`, `{"id":"old","is_minor":null}`, `{"id":"old","is_minor":true}`} {
		var u User
		if err := json.Unmarshal([]byte(input), &u); err != nil || !u.IsMinor {
			t.Fatal("旧用户未默认按未成年处理", err)
		}
	}
	var u User
	if err := json.Unmarshal([]byte(`{"is_minor":false}`), &u); err != nil || u.IsMinor {
		t.Fatal("明确成年标记未保留", err)
	}
}

// TestIdentitySettingsEncryption 验证随机化、完整性、独立用途及缺少部署密钥时拒绝保存。
func TestIdentitySettingsEncryption(t *testing.T) {
	setPaymentEncryptionKey(t)
	in := identitySettings{APIKey: "identity-test-secret"}
	a, err := sealIdentitySettings(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sealIdentitySettings(in)
	if err != nil || a.Ciphertext == b.Ciphertext {
		t.Fatal("核验配置未随机加密", err)
	}
	if out, err := openIdentitySettings(a); err != nil || out != in {
		t.Fatal("合法密文不能还原", err)
	}
	if _, err := openPaymentSettings(a); err == nil {
		t.Fatal("核验密文被当作支付配置接受")
	}
	raw, _ := base64.StdEncoding.DecodeString(a.Ciphertext)
	raw[len(raw)-1] ^= 1
	a.Ciphertext = base64.StdEncoding.EncodeToString(raw)
	if _, err := openIdentitySettings(a); err == nil {
		t.Fatal("篡改密文被接受")
	}
	t.Setenv("CONFIG_ENCRYPTION_KEY", "")
	if _, err := sealIdentitySettings(in); err == nil {
		t.Fatal("无部署密钥仍保存核验秘密")
	}
}

// TestIdentityProviderProtocol 验证固定目标、表单及失败提示，不将成功请求但不一致误判为通过。
func TestIdentityProviderProtocol(t *testing.T) {
	card := testIdentityCard("19930616")
	for _, test := range []struct {
		name, body, message string
		status              int
		success             bool
	}{{"一致", `{"code":200,"msg":"数据请求成功","data":{"result":true},"request_id":"test-request-1"}`, "", 200, true}, {"不一致", `{"code":200,"msg":"数据请求成功","data":{"result":false,"desc":"不一致"},"request_id":"test-request-2"}`, "不一致", 200, false}, {"错误提示", `{"code":403,"msg":"余额不足"}`, "余额不足", 200, false}, {"HTTP错误", `{"code":200,"msg":"服务故障","data":{"result":true}}`, "服务故障", 503, false}, {"缺少结果", `{"code":200,"data":{},"request_id":"r"}`, "姓名与身份证号不一致", 200, false}, {"错误结果类型", `{"code":200,"data":{"result":"true"},"request_id":"r"}`, "返回异常", 200, false}, {"损坏响应", `not-json`, "返回异常", 200, false}, {"缺少流水", `{"code":200,"data":{"result":true}}`, "返回异常", 200, false}, {"响应过长", strings.Repeat("x", 32769), "返回异常", 200, false}} {
		t.Run(test.name, func(t *testing.T) {
			app := &App{HTTPClient: &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != identityAPIURL || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fake-test-key" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Fatal("请求目标或请求头不符合协议")
				}
				if err := r.ParseForm(); err != nil || r.Form.Get("name") != "合成测试" || r.Form.Get("idcard") != card {
					t.Fatal("请求表单不符合协议")
				}
				return &http.Response{StatusCode: test.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(test.body)), Request: r}, nil
			})}}
			id, err := app.callIdentityCheck(t.Context(), "fake-test-key", "合成测试", card)
			if (err == nil) != test.success || test.success && id != "test-request-1" {
				t.Fatalf("一致性判定错误 id=%q err=%v", id, err)
			}
			if !test.success && !strings.Contains(err.Error(), test.message) {
				t.Fatalf("未保留可读失败原因: %v", err)
			}
		})
	}
	var calls int
	app := &App{HTTPClient: &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 307, Header: http.Header{"Location": []string{"https://other.invalid/leak"}}, Body: io.NopCloser(strings.NewReader(`{"msg":"请求跳转"}`)), Request: r}, nil
	})}}
	if _, err := app.callIdentityCheck(t.Context(), "secret", "合成测试", card); err == nil || calls != 1 {
		t.Fatal("重定向未被拒绝")
	}
	app.HTTPClient.Transport = paymentTransport(func(r *http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
	if _, err := app.callIdentityCheck(t.Context(), "secret", "合成测试", card); err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatal("超时未给出明确提示")
	}
}

// mockIdentityProvider 安装仅接受固定核验端点的协议替身，不产生真实网络请求或收费。
func mockIdentityProvider(t *testing.T, api *testAPI, body string, calls *atomic.Int32) {
	t.Helper()
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != identityAPIURL {
			return nil, errors.New("拒绝非核验测试目标")
		}
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
}

// TestIntegrationIdentityVerificationAndPrivacy 验证权限、密钥不回显、未成年零调用及真实数据库原子通过状态。
func TestIntegrationIdentityVerificationAndPrivacy(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	setTestAdultIdentity(t, api, "player", false)
	api.call("player", "GET", "/admin/identity-settings", nil, 403, nil)
	api.call("player", "PUT", "/admin/identity-settings", map[string]string{"api_key": "fake-test-secret"}, 403, nil)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"api_key": "fake-test-secret"}, 200, nil)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"api_key": ""}, 200, nil)
	for _, path := range []string{"/admin/identity-settings", "/me/identity"} {
		_, raw, err := api.request("admin", "GET", path, nil)
		if err != nil || strings.Contains(string(raw), "fake-test-secret") {
			t.Fatal("响应泄露密钥", err)
		}
	}
	stored, err := readIdentitySettings(t.Context(), api.App.DB)
	if err != nil || stored.APIKey != "fake-test-secret" {
		t.Fatal("空输入未保留秘密", err)
	}
	var ciphertext []byte
	if err = api.App.DB.QueryRow(t.Context(), "SELECT data FROM settings WHERE id='identity-private'").Scan(&ciphertext); err != nil || strings.Contains(string(ciphertext), stored.APIKey) {
		t.Fatal("数据库保存了明文密钥", err)
	}
	var calls atomic.Int32
	mockIdentityProvider(t, api, `{"code":200,"msg":"成功","data":{"result":true},"request_id":"local-test-request"}`, &calls)
	api.call("player", "POST", "/me/identity/verify", map[string]any{"name": "合成测试", "idcard": testIdentityCard("20150101"), "consent": true}, 403, nil)
	api.call("player", "POST", "/me/identity/verify", map[string]any{"name": "合成测试", "idcard": "invalid", "consent": true}, 400, nil)
	api.call("player", "POST", "/me/identity/verify", map[string]any{"name": "合成测试", "idcard": testIdentityCard("19930616"), "consent": false}, 400, nil)
	if calls.Load() != 0 {
		t.Fatal("未成年或非法输入触发收费接口")
	}
	var status IdentityStatus
	input := map[string]any{"name": "合成测试", "idcard": testIdentityCard("19930616"), "consent": true}
	api.call("player", "POST", "/me/identity/verify", input, 200, &status)
	api.call("player", "POST", "/me/identity/verify", input, 200, nil)
	if !status.Verified || status.IsMinor || status.VerifiedAt == "" || calls.Load() != 1 {
		t.Fatal("成功核验未持久化或重复收费")
	}
	u, err := get[User](t.Context(), api.App.DB, "users", "player", false)
	identity, identityErr := get[map[string]string](t.Context(), api.App.DB, "identities", "player", false)
	private, _ := json.Marshal(identity)
	if err != nil || identityErr != nil || u.IsMinor || identity["phone_number"] == "" || strings.Contains(string(private), "合成测试") || strings.Contains(string(private), input["idcard"].(string)) {
		t.Fatal("核验事实或隐私保存错误", err, identityErr)
	}
	api.call("player", "PATCH", "/me", map[string]any{"is_minor": false}, 400, nil)
}

// TestIntegrationIdentityPaymentGates 验证历史订单、余额支付、补款及所有充值押金在每次发起时实时查询用户表。
func TestIntegrationIdentityPaymentGates(t *testing.T) {
	api := newTestAPI(t)
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	adjustment := OrderAdjustment{ID: "identity-gate-adjustment", OrderID: order.ID, Kind: "increase", Status: "awaiting_payment"}
	if err := put(t.Context(), api.App.DB, "order_adjustments", adjustment.ID, adjustment); err != nil {
		t.Fatal(err)
	}
	topup := WalletTopup{ID: "identity-old-topup", UserID: "player", Status: "pending_payment", Purpose: "balance", AmountCents: 100}
	if err := put(t.Context(), api.App.DB, "wallet_topups", topup.ID, topup); err != nil {
		t.Fatal(err)
	}
	setTestAdultIdentity(t, api, "player", false)
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 428, nil)
	api.call("player", "POST", "/requests", map[string]any{}, 428, nil)
	for _, channel := range []string{"wechat", "balance"} {
		api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{"channel": channel, "request_id": "identity-gate-" + channel}, 428, nil)
		api.call("player", "POST", "/orders/"+order.ID+"/adjustments/"+adjustment.ID+"/pay", map[string]any{"channel": channel, "request_id": "identity-part-" + channel}, 428, nil)
	}
	for _, purpose := range []string{"balance", "deposit", "club_platform", "member_club"} {
		api.call("player", "POST", "/wallet/topups", map[string]any{"purpose": purpose, "amount_cents": 100, "request_id": "identity-gate-" + purpose}, 428, nil)
	}
	api.call("player", "POST", "/wallet/topups/"+topup.ID+"/pay", nil, 428, nil)
	// 只伪造用户表false而没有私密事实，或只保存私密事实而表仍为true，均不能付款。
	u, _ := get[User](t.Context(), api.App.DB, "users", "player", false)
	u.IsMinor = false
	put(t.Context(), api.App.DB, "users", "player", u)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", nil, 428, nil)
	setTestAdultIdentity(t, api, "player", true)
	u.IsMinor = true
	put(t.Context(), api.App.DB, "users", "player", u)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", nil, 428, nil)
	setTestAdultIdentity(t, api, "player", true)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", nil, 200, nil)
	// 已付款回执、查询原单和退款不因后续未核验标记而被资金门禁阻断。
	setTestAdultIdentity(t, api, "player", false)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", nil, 200, nil)
	api.call("player", "GET", "/orders/"+order.ID, nil, 200, nil)
}

// TestIntegrationIdentityFailureRateAndMigration 验证不一致不落库、供应商msg、频率上限及历史用户保守迁移。
func TestIntegrationIdentityFailureRateAndMigration(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	setTestAdultIdentity(t, api, "player", false)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"api_key": "offline-test-key"}, 200, nil)
	var calls atomic.Int32
	mockIdentityProvider(t, api, `{"code":200,"msg":"数据请求成功","data":{"result":false,"desc":"姓名与证件不一致"},"request_id":"not-matched"}`, &calls)
	input := map[string]any{"name": "合成测试", "idcard": testIdentityCard("19930616"), "consent": true}
	for i := 0; i < 3; i++ {
		code, raw, err := api.request("player", "POST", "/me/identity/verify", input)
		if err != nil || code != 422 || !strings.Contains(string(raw), "姓名与证件不一致") {
			t.Fatalf("未正确显示不一致原因 %d %s %v", code, raw, err)
		}
	}
	api.call("player", "POST", "/me/identity/verify", input, 429, nil)
	if calls.Load() != 3 {
		t.Fatal("限流后仍调用上游")
	}
	var status IdentityStatus
	api.call("player", "GET", "/me/identity", nil, 200, &status)
	if status.Verified || !status.IsMinor {
		t.Fatal("核验失败错误放行")
	}
	if _, err := api.App.DB.Exec(t.Context(), "UPDATE users SET data=data-'is_minor' WHERE id='player'"); err != nil {
		t.Fatal(err)
	}
	if err := migrateIdentityVerification(t.Context(), api.App.DB); err != nil {
		t.Fatal(err)
	}
	var stored bool
	if err := api.App.DB.QueryRow(t.Context(), "SELECT (data->>'is_minor')::boolean FROM users WHERE id='player'").Scan(&stored); err != nil || !stored {
		t.Fatal("历史用户迁移未保守默认true", err)
	}
	mockIdentityProvider(t, api, `{"code":403,"msg":"接口余额不足"}`, &calls)
	setTestAdultIdentity(t, api, "outsider", false)
	code, raw, err := api.request("outsider", "POST", "/me/identity/verify", input)
	if err != nil || code != 502 || !strings.Contains(string(raw), "接口余额不足") {
		t.Fatal(fmt.Sprintf("提供方msg未显示 %d %s %v", code, raw, err))
	}
}

// TestIntegrationIdentityConcurrentRequests 验证同账号并发请求只发送一次核验，其余明确提示处理中。
func TestIntegrationIdentityConcurrentRequests(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	setTestAdultIdentity(t, api, "player", false)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"api_key": "offline-concurrent-key"}, 200, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != identityAPIURL {
			return nil, errors.New("拒绝非核验测试目标")
		}
		calls.Add(1)
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":200,"data":{"result":true},"request_id":"concurrent-request"}`)), Request: r}, nil
	})}
	input := map[string]any{"name": "合成测试", "idcard": testIdentityCard("19930616"), "consent": true}
	done := make(chan error, 1)
	go func() {
		code, raw, err := api.request("player", "POST", "/me/identity/verify", input)
		if err == nil && code != 200 {
			err = fmt.Errorf("首个核验响应错误 %d %s", code, raw)
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("核验请求未进入替身")
	}
	code, raw, err := api.request("player", "POST", "/me/identity/verify", input)
	close(release)
	if err != nil || code != 429 || !strings.Contains(string(raw), "正在核验") {
		t.Fatalf("并发核验未明确拒绝 %d %s %v", code, raw, err)
	}
	if err = <-done; err != nil || calls.Load() != 1 {
		t.Fatal("并发请求重复调用提供方", err)
	}
}
