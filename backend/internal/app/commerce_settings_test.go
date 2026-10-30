// Package app 验证渠道配置的加密、权限、费率边界以及跨入口旧交易配置恢复。
package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCommerceSMSCapabilityProjection 保证保存配置的响应不会丢失已启用的短信能力或泄露凭据。
func TestCommerceSMSCapabilityProjection(t *testing.T) {
	for _, key := range []string{"TENCENT_SMS_SECRET_ID", "TENCENT_SMS_SECRET_KEY", "TENCENT_SMS_SDK_APP_ID", "TENCENT_SMS_SIGN_NAME", "TENCENT_SMS_TEMPLATE_ID"} {
		t.Setenv(key, "test-sms-capability-value")
	}
	a := &App{}
	public, saved := a.commercePublicView(), a.commerceSettingsView(a.commerceSettings())
	if public["sms_configured"] != true || saved["sms_configured"] != true {
		t.Fatal("保存响应丢失了公开短信能力状态")
	}
	raw, _ := json.Marshal(saved)
	if strings.Contains(string(raw), "test-sms-capability-value") {
		t.Fatal("能力投影泄露短信凭据")
	}
	t.Setenv("TENCENT_SMS_SECRET_KEY", "")
	if a.commerceSettingsView(a.commerceSettings())["sms_configured"] != false {
		t.Fatal("缺少短信密钥仍标记可用")
	}
}

// TestCommerceDefaultsAndValidation 验证平台默认承担费用且非法费率与未知模式不能保存。
func TestCommerceDefaultsAndValidation(t *testing.T) {
	for _, key := range []string{"WECHAT_COMMERCE_MODE", "WECHAT_VIRTUAL_ENV", "WECHAT_VIRTUAL_OFFER_ID", "WECHAT_OFFICIAL_APP_ID", "WECHAT_OFFICIAL_REDIRECT_URL"} {
		t.Setenv(key, "")
	}
	v := (&App{}).commerceSettings()
	if v.Mode != "mini_program" || v.FeeBearer != "platform" || v.AndroidFeeBPS != 0 || v.IOSFeeBPS != 0 {
		t.Fatal("默认渠道或费用承担方错误")
	}
	if err := validateCommerceSettings(v); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []commerceSettings{
		{Mode: "unknown", FeeBearer: "platform"},
		{Mode: "mini_program", FeeBearer: "unknown"},
		{Mode: "mini_program", FeeBearer: "buyer", AndroidFeeBPS: -1},
		{Mode: "mini_program", FeeBearer: "platform", IOSFeeBPS: 10000},
		{Mode: "mini_program", FeeBearer: "platform", VirtualEnv: 2},
		{Mode: "mini_program", FeeBearer: "platform", OfferID: "123&extra=1"},
		{Mode: "official_account", FeeBearer: "platform", OfficialAppID: "invalid"},
		{Mode: "official_account", FeeBearer: "platform", OfficialRedirectURL: "https://user:secret@example.com/callback"},
	} {
		if validateCommerceSettings(invalid) == nil {
			t.Fatal("非法渠道配置被接受")
		}
	}
}

// TestCommerceEncryptionIsolation 验证秘密随机加密且不能把普通商户密文替换为渠道密文。
func TestCommerceEncryptionIsolation(t *testing.T) {
	setPaymentEncryptionKey(t)
	v := commerceSettings{Mode: "mini_program", FeeBearer: "platform", AppKey: strings.Repeat("A", 32)}
	first, err := sealCommerceSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sealCommerceSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	if first.Ciphertext == second.Ciphertext || strings.Contains(first.Ciphertext, v.AppKey) {
		t.Fatal("配置没有随机加密")
	}
	loaded, err := openCommerceSettings(first)
	if err != nil || loaded.AppKey != v.AppKey {
		t.Fatal("无法恢复已保存的渠道配置")
	}
	ordinary, err := sealPaymentSettings(paymentSettings{APIKey: v.AppKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = openCommerceSettings(ordinary); err == nil {
		t.Fatal("接受了不同用途的密文")
	}
	app := &App{Config: Config{WechatAppID: "wx1234567890abcdef"}, commerceConfig: &v}
	raw, _ := json.Marshal(app.commercePublicView())
	admin, _ := json.Marshal(app.commerceSettingsView(v))
	if strings.Contains(string(raw), v.AppKey) || strings.Contains(string(admin), v.AppKey) {
		t.Fatal("渠道秘密出现在接口响应中")
	}
}

// TestIntegrationCommerceSettings 覆盖管理员保存、秘密保留、重载、模式切换和旧AppID归属恢复。
func TestIntegrationCommerceSettings(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	c, _ := testPayConfig(t)
	api.App.Config.WechatAppID = c.AppID
	api.App.Config.WechatMessageToken = "only-test-callback-token"
	input := testPaymentSettingsInput(t, c)
	api.call("admin", "PATCH", "/admin/payment-settings", input, 200, nil)
	api.call("", "GET", "/admin/commerce-settings", nil, 401, nil)
	api.call("worker", "GET", "/admin/commerce-settings", nil, 403, nil)
	api.call("worker", "PATCH", "/admin/commerce-settings", map[string]string{"mode": "official_account"}, 403, nil)
	secret := strings.Repeat("v", 32)
	firstAppID, secondAppID := "wx1111111111111111", "wx2222222222222222"
	var safe map[string]any
	api.call("admin", "PATCH", "/admin/commerce-settings", map[string]any{"mode": "official_account", "fee_bearer": "buyer", "android_fee_bps": 100, "ios_fee_bps": 1200, "offer_id": "12345", "app_key": secret, "official_app_id": firstAppID, "official_secret": secret, "official_redirect_url": "https://example.com/#/pages/account/login"}, 400, nil)
	api.call("admin", "PATCH", "/admin/commerce-settings", map[string]any{"mode": "official_account", "fee_bearer": "buyer", "android_fee_bps": 100, "ios_fee_bps": 1200, "offer_id": "12345", "app_key": secret, "official_app_id": firstAppID, "official_secret": secret, "official_redirect_url": "https://example.com/"}, 200, &safe)
	if safe["official_configured"] != true || safe["virtual_configured"] != true || safe["official_secret_set"] != true {
		t.Fatal("配置状态不符合实际字段")
	}
	api.call("admin", "PATCH", "/admin/commerce-settings", map[string]any{"mode": "mini_program", "official_app_id": secondAppID, "official_secret": "", "app_key": ""}, 200, nil)
	loaded := api.App.commerceSettings()
	if loaded.OfficialSecret != secret || loaded.AppKey != secret || !contains(loaded.PreviousOfficialAppIDs, firstAppID) {
		t.Fatal("空秘密输入或模式切换丢失既有配置")
	}
	for _, appID := range []string{c.AppID, firstAppID, secondAppID} {
		cfg, err := api.App.loadWXPayForApp(appID)
		if err != nil || cfg.AppID != appID || cfg.MerchantID != c.MerchantID {
			t.Fatal("旧交易无法按原AppID恢复")
		}
	}
	if _, err := api.App.loadWXPayForApp("wx9999999999999999"); err == nil {
		t.Fatal("任意AppID可以使用商户签名")
	}
	api.call("admin", "PATCH", "/admin/commerce-settings", map[string]int{"ios_fee_bps": 10000}, 400, nil)
	if api.App.commerceSettings().IOSFeeBPS != 1200 {
		t.Fatal("失败保存破坏既有费率")
	}
	for _, path := range []string{"/admin/commerce-settings", "/bootstrap", "/admin/settings", "/admin/audit"} {
		status, raw, err := api.request("admin", "GET", path, nil)
		if err != nil || status != 200 || strings.Contains(string(raw), secret) {
			t.Fatal("配置响应泄露秘密或读取失败")
		}
	}
	var stored string
	if err := api.App.DB.QueryRow(t.Context(), "SELECT data::text FROM settings WHERE id='commerce-private'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, secret) || strings.Contains(stored, "official_secret") || !strings.Contains(stored, "ciphertext") {
		t.Fatal("渠道资料没有整份加密保存")
	}
	restarted, err := New(t.Context(), api.App.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.commerceSettings().AppKey != secret || restarted.commerceSettings().OfficialAppID != secondAppID {
		t.Fatal("应用重启丢失渠道配置")
	}
}
