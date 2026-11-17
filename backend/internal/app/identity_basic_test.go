// Package app 验证基础校验的格式、生日边界、零第三方调用和切回严格后的交易约束。
package app

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestIdentityBasicValidation 验证基础方式跳过校验位但保留十八位格式和真实出生日期。
func TestIdentityBasicValidation(t *testing.T) {
	at := time.Date(2026, 11, 17, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	for _, test := range []struct {
		card         string
		valid, adult bool
	}{
		{"00000020081117000X", true, true}, {"00000020081118000X", true, false},
		{"00000020000230000X", false, false}, {"00000020271117000X", false, false},
		{"00000A19900101000X", false, false}, {"00000019900101000A", false, false},
		{"00000019900101000", false, false},
	} {
		adult, err := identityAdultWithMode(test.card, at, true)
		if (err == nil) != test.valid || adult != test.adult {
			t.Fatalf("基础边界未通过 valid=%v adult=%v error=%v", test.valid, adult, err)
		}
	}
	if _, err := identityAdult("00000020081117000X", at); err == nil {
		t.Fatal("严格模式不能接受基础格式夹具")
	}
}

// TestIntegrationIdentityBasicMode 只操作独立测试schema，基础通过不调用供应商，切回严格实时收紧。
func TestIntegrationIdentityBasicMode(t *testing.T) {
	t.Setenv("CONFIG_ENCRYPTION_KEY", "")
	api := newTestAPI(t)
	setTestAdultIdentity(t, api, "player", false)
	var calls atomic.Int32
	mockIdentityProvider(t, api, `{"code":200,"data":{"result":true},"request_id":"strict-test"}`, &calls)
	api.call("player", "PUT", "/admin/identity-settings", map[string]string{"mode": "basic"}, 403, nil)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"mode": "unknown"}, 400, nil)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"mode": "basic", "api_key": ""}, 200, nil)
	for _, name := range []string{"张", strings.Repeat("张", 6)} {
		api.call("player", "POST", "/me/identity/verify", map[string]any{"name": name, "idcard": "00000019900101000x", "consent": true}, 400, nil)
	}
	api.call("player", "POST", "/me/identity/verify", map[string]any{"name": "合成测试", "idcard": "00000020150101000X", "consent": true}, 403, nil)
	var status IdentityStatus
	api.call("player", "POST", "/me/identity/verify", map[string]any{"name": "合成测试", "idcard": "00000019900101000x", "consent": true}, 200, &status)
	if !status.Verified || status.VerificationSource != identityBasic || status.VerificationMode != identityBasic || calls.Load() != 0 {
		t.Fatal("基础方式必须独立通过且零第三方调用", status, calls.Load())
	}
	private, err := get[map[string]string](t.Context(), api.App.DB, "identities", "player", false)
	if err != nil || private["identity_source"] != identityBasic || strings.Contains(private["identity_name_masked"], "合成测试") {
		t.Fatal("来源或隐私投影错误", err)
	}
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"mode": "strict", "api_key": ""}, 200, nil)
	api.call("player", "GET", "/me/identity", nil, 200, &status)
	if status.Verified || status.VerificationMode != identityStrict {
		t.Fatal("基础记录不能越过已切回的严格规则", status)
	}
	if _, err = api.App.storeAdultIdentity(t.Context(), "player", "合成测试", "00000019900101000X", "", identityBasic); err == nil {
		t.Fatal("基础迟到提交必须核对当前模式")
	}
	setTestAdultIdentity(t, api, "player", true)
	api.call("player", "GET", "/me/identity", nil, 200, &status)
	if !status.Verified || status.VerificationSource != "xxapi" {
		t.Fatal("既有API通过事实应保留", status)
	}
}

// TestIntegrationIdentityModePreservesSecret 验证密钥不可解密时仍能切换基础方式，原密文不会被空输入覆盖。
func TestIntegrationIdentityModePreservesSecret(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"api_key": "synthetic-test-key"}, 200, nil)
	before, err := get[encryptedPaymentSettings](t.Context(), api.App.DB, "settings", "identity-private", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_ENCRYPTION_KEY", "")
	api.call("admin", "PUT", "/admin/identity-settings", map[string]string{"mode": "basic", "api_key": ""}, 200, nil)
	var view map[string]any
	api.call("admin", "GET", "/admin/identity-settings", nil, 200, &view)
	if view["configured"] != true || view["mode"] != "basic" || view["storage_available"] != false {
		t.Fatal("基础方式不能依赖加密密钥", view)
	}
	after, err := get[encryptedPaymentSettings](t.Context(), api.App.DB, "settings", "identity-private", false)
	if err != nil || before != after {
		t.Fatal("切换方式不能改写旧密文", err)
	}
}
