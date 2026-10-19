// Package app 验证管理支付配置的加密落库、秘密不回显、权限、重载和并发快照读取。
package app

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"sync"
	"testing"
	"time"
)

// setPaymentEncryptionKey 为每个测试生成独立的数据库配置加密密钥。
func setPaymentEncryptionKey(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
}

// testPaymentSettingsInput 把一次性测试 RSA 密钥转换为管理员输入格式。
func testPaymentSettingsInput(t *testing.T, c wxPayConfig) paymentSettings {
	t.Helper()
	public, err := x509.MarshalPKIXPublicKey(c.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return paymentSettings{MerchantID: c.MerchantID, CertificateSerial: c.Serial, PublicKeyID: c.PublicKeyID, NotifyURL: c.NotifyURL, RefundNotifyURL: c.RefundNotifyURL, APIKey: c.APIKey, PrivatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(c.PrivateKey)})), PublicPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}))}
}

// TestPaymentEncryptionIntegrity 覆盖每次加密随机化、错主密钥、篡改密文和缺少部署密钥时的拒绝。
func TestPaymentEncryptionIntegrity(t *testing.T) {
	setPaymentEncryptionKey(t)
	input := paymentSettings{MerchantID: "merchant-test", APIKey: strings.Repeat("k", 32)}
	first, err := sealPaymentSettings(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sealPaymentSettings(input)
	if err != nil || first.Ciphertext == second.Ciphertext {
		t.Fatal("密文未使用独立随机nonce")
	}
	decoded, err := openPaymentSettings(first)
	if err != nil || decoded != input {
		t.Fatal("合法配置不能解密")
	}
	raw, _ := base64.StdEncoding.DecodeString(first.Ciphertext)
	raw[len(raw)-1] ^= 1
	tampered := first
	tampered.Ciphertext = base64.StdEncoding.EncodeToString(raw)
	if _, err = openPaymentSettings(tampered); err == nil {
		t.Fatal("篡改密文被接受")
	}
	setPaymentEncryptionKey(t)
	if _, err = openPaymentSettings(first); err == nil {
		t.Fatal("错主密钥被接受")
	}
	t.Setenv("CONFIG_ENCRYPTION_KEY", "")
	if _, err = sealPaymentSettings(input); err == nil {
		t.Fatal("未配置加密主密钥仍存储支付秘密")
	}
}

// TestIntegrationPaymentSettingsSecretsAndReload 覆盖权限、草稿、秘密保留、落库密文、重启加载和配置验证。
func TestIntegrationPaymentSettingsSecretsAndReload(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	c, _ := testPayConfig(t)
	api.App.Config.WechatAppID = c.AppID
	input := testPaymentSettingsInput(t, c)
	api.call("", "GET", "/admin/payment-settings", nil, 401, nil)
	api.call("worker", "GET", "/admin/payment-settings", nil, 403, nil)
	api.call("worker", "PATCH", "/admin/payment-settings", input, 403, nil)
	api.call("clubowner", "PATCH", "/admin/payment-settings", input, 403, nil)
	api.call("admin", "GET", "/admin/payment-private", nil, 404, nil)
	var safe map[string]any
	api.call("admin", "PATCH", "/admin/payment-settings", map[string]string{"merchant_id": input.MerchantID}, 200, &safe)
	if safe["configured"] != false || safe["storage_available"] != true {
		t.Fatal("部分商户配置不能标为可用")
	}
	if _, err := api.App.loadWXPay(); err == nil {
		t.Fatal("部分配置进入真实支付")
	}
	api.call("admin", "PATCH", "/admin/payment-settings", input, 200, &safe)
	if safe["configured"] != true || safe["api_v3_key_set"] != true || safe["merchant_private_key_set"] != true || safe["platform_public_key_set"] != true {
		t.Fatal("完整合法配置未就绪")
	}
	api.call("admin", "PATCH", "/admin/payment-settings", map[string]string{"api_v3_key": "", "merchant_private_key_pem": "", "platform_public_key_pem": "", "notify_url": "https://new.example.com/payment"}, 200, &safe)
	runtime, err := api.App.loadWXPay()
	if err != nil || runtime.APIKey != input.APIKey || runtime.MerchantID != input.MerchantID || runtime.NotifyURL != "https://new.example.com/payment" {
		t.Fatal("部分保存或空输入覆盖既有秘密")
	}
	for _, body := range []map[string]string{{"api_v3_key": "short"}, {"merchant_private_key_pem": "invalid"}, {"platform_public_key_pem": "invalid"}, {"notify_url": "http://example.com/payment"}, {"refund_notify_url": "https://user:password@example.com/payment"}, {"merchant_id": "bad\nheader"}} {
		api.call("admin", "PATCH", "/admin/payment-settings", body, 400, nil)
	}
	if _, err = api.App.loadWXPay(); err != nil {
		t.Fatal("无效输入破坏此前有效配置")
	}
	for _, path := range []string{"/admin/payment-settings", "/admin/audit", "/admin/settings"} {
		code, raw, e := api.request("admin", "GET", path, nil)
		if code != 200 || e != nil {
			t.Fatalf("配置读取错误 %d %v", code, e)
		}
		for _, secret := range []string{input.APIKey, "BEGIN RSA PRIVATE KEY", "BEGIN PUBLIC KEY", "\"api_v3_key\":", "\"merchant_private_key_pem\":"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("%s 泄露秘密字段 %s", path, secret)
			}
		}
	}
	var stored string
	if err = api.App.DB.QueryRow(t.Context(), "SELECT data::text FROM settings WHERE id='payment-private'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, input.APIKey) || strings.Contains(stored, "PRIVATE KEY") || strings.Contains(stored, "merchant_id") || !strings.Contains(stored, "ciphertext") {
		t.Fatal("数据库支付配置未全量加密")
	}
	restarted, err := New(t.Context(), api.App.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	loaded, err := restarted.loadWXPay()
	if err != nil || loaded.APIKey != input.APIKey || loaded.NotifyURL != runtime.NotifyURL || loaded.PrivateKey.N.Cmp(c.PrivateKey.N) != 0 {
		t.Fatal("重建应用不能恢复配置")
	}
	api.call("admin", "PATCH", "/admin/payment-settings", map[string]string{"merchant_id": ""}, 200, &safe)
	if safe["configured"] != false {
		t.Fatal("清空必填商户号未关闭支付")
	}
	if _, err = api.App.loadWXPay(); err == nil {
		t.Fatal("清空配置后支付仍启用")
	}
}

// TestIntegrationPaymentConfigSnapshotConcurrent 验证交易持锁读取不访问数据库，并发管理保存不会产生竞态或混合版本。
func TestIntegrationPaymentConfigSnapshotConcurrent(t *testing.T) {
	setPaymentEncryptionKey(t)
	api := newTestAPI(t)
	c, _ := testPayConfig(t)
	api.App.Config.WechatAppID = c.AppID
	input := testPaymentSettingsInput(t, c)
	api.call("admin", "PATCH", "/admin/payment-settings", input, 200, nil)
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	done := make(chan error, 1)
	go func() { _, e := api.App.loadWXPay(); done <- e }()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("支付配置读取被数据库事务阻塞")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				cfg, e := api.App.loadWXPay()
				if e != nil || cfg.APIKey != c.APIKey || cfg.PrivateKey == nil || cfg.PublicKey == nil {
					t.Error("并发读取出现半份支付配置")
				}
			}
		}()
	}
	for i := 0; i < 3; i++ {
		api.call("admin", "PATCH", "/admin/payment-settings", map[string]string{"notify_url": "https://example.com/updated-notify"}, 200, nil)
	}
	wg.Wait()
	// 支付配置的响应只能包含显式脱敏字段，禁止未来序列化整个内部结构。
	var safe map[string]json.RawMessage
	api.call("admin", "GET", "/admin/payment-settings", nil, 200, &safe)
	allowed := map[string]bool{"merchant_id": true, "certificate_serial": true, "platform_public_key_id": true, "notify_url": true, "refund_notify_url": true, "api_v3_key_set": true, "merchant_private_key_set": true, "platform_public_key_set": true, "configured": true, "source": true, "storage_available": true}
	for key := range safe {
		if !allowed[key] {
			t.Fatalf("意外暴露支付字段: %s", key)
		}
	}
}
