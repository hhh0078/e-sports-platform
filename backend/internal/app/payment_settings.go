// Package app 加密保存管理员支付配置，并以只读内存快照向交易流程提供线程安全的配置。
package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

type paymentSettings struct {
	MerchantID        string `json:"merchant_id"`
	CertificateSerial string `json:"certificate_serial"`
	PublicKeyID       string `json:"platform_public_key_id"`
	NotifyURL         string `json:"notify_url"`
	RefundNotifyURL   string `json:"refund_notify_url"`
	APIKey            string `json:"api_v3_key"`
	PrivatePEM        string `json:"merchant_private_key_pem"`
	PublicPEM         string `json:"platform_public_key_pem"`
}
type paymentConfigState struct {
	Settings paymentSettings
	Runtime  wxPayConfig
	Ready    bool
}
type encryptedPaymentSettings struct {
	Version    int    `json:"version"`
	Ciphertext string `json:"ciphertext"`
}

// paymentCipher 从部署环境读取独立的三十二字节加密主密钥，不使用微信商户密钥作数据库密钥。
func paymentCipher() (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv("CONFIG_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		return nil, fail(503, "服务器尚未配置支付资料加密密钥，请联系运维配置 CONFIG_ENCRYPTION_KEY")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealPaymentSettings 用随机 nonce 和固定用途标签加密支付配置，数据库不会存储明文秘密。
func sealPaymentSettings(value paymentSettings) (encryptedPaymentSettings, error) {
	gcm, err := paymentCipher()
	if err != nil {
		return encryptedPaymentSettings{}, err
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return encryptedPaymentSettings{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedPaymentSettings{}, err
	}
	result := gcm.Seal(nonce, nonce, plain, []byte("jijie-payment-settings-v1"))
	return encryptedPaymentSettings{Version: 1, Ciphertext: base64.StdEncoding.EncodeToString(result)}, nil
}

// openPaymentSettings 校验加密数据完整性，损坏或主密钥错误时关闭支付配置加载。
func openPaymentSettings(value encryptedPaymentSettings) (paymentSettings, error) {
	var result paymentSettings
	gcm, err := paymentCipher()
	if err != nil {
		return result, err
	}
	raw, err := base64.StdEncoding.DecodeString(value.Ciphertext)
	if err != nil || value.Version != 1 || len(raw) < gcm.NonceSize() {
		return result, errors.New("支付配置密文格式无效")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte("jijie-payment-settings-v1"))
	if err != nil {
		return result, errors.New("支付配置无法解密，请核对部署加密密钥")
	}
	if json.Unmarshal(plain, &result) != nil {
		return result, errors.New("支付配置数据格式无效")
	}
	return result, nil
}

// parseMerchantKey 读取标准 RSA 私钥并拒绝弱密钥和损坏的私钥参数。
func parseMerchantKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, fail(400, "商户私钥必须是有效 PEM 格式")
	}
	var key *rsa.PrivateKey
	if parsed, e := x509.ParsePKCS8PrivateKey(block.Bytes); e == nil {
		key, _ = parsed.(*rsa.PrivateKey)
	} else {
		key, _ = x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	if key == nil || key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, fail(400, "商户私钥必须为有效的至少 2048 位 RSA 密钥")
	}
	return key, nil
}

// parsePlatformKey 读取微信支付平台 RSA 公钥或证书，仅接受至少二千零四十八位密钥。
func parsePlatformKey(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, fail(400, "微信支付公钥必须是有效 PEM 格式")
	}
	var key *rsa.PublicKey
	if parsed, e := x509.ParsePKIXPublicKey(block.Bytes); e == nil {
		key, _ = parsed.(*rsa.PublicKey)
	} else if cert, e := x509.ParseCertificate(block.Bytes); e == nil {
		key, _ = cert.PublicKey.(*rsa.PublicKey)
	}
	if key == nil || key.N.BitLen() < 2048 {
		return nil, fail(400, "微信支付公钥必须为至少 2048 位 RSA 密钥")
	}
	return key, nil
}

// validatePaymentDraft 允许缺项草稿，但任何已填写字段都必须满足真实支付的格式要求。
func validatePaymentDraft(v paymentSettings) error {
	for _, id := range []string{v.MerchantID, v.CertificateSerial, v.PublicKeyID} {
		if len(id) > 128 || strings.ContainsAny(id, "\r\n\t \"\\") {
			return fail(400, "商户号和证书标识格式无效")
		}
	}
	if v.APIKey != "" && len(v.APIKey) != 32 {
		return fail(400, "APIv3 密钥必须恰好 32 字节")
	}
	for _, address := range []string{v.NotifyURL, v.RefundNotifyURL} {
		if address != "" && (len(address) > 2048 || !validNotifyURL(address)) {
			return fail(400, "支付和退款通知地址必须为有效 HTTPS URL")
		}
	}
	if v.PrivatePEM != "" {
		if _, err := parseMerchantKey(v.PrivatePEM); err != nil {
			return err
		}
	}
	if v.PublicPEM != "" {
		if _, err := parsePlatformKey(v.PublicPEM); err != nil {
			return err
		}
	}
	return nil
}

// runtimePaymentConfig 构造完整的商户配置，未填齐或无效的配置绝不进入真实扣款流程。
func (a *App) runtimePaymentConfig(v paymentSettings) (wxPayConfig, error) {
	c := wxPayConfig{AppID: a.Config.WechatAppID, MerchantID: v.MerchantID, Serial: v.CertificateSerial, PublicKeyID: v.PublicKeyID, APIKey: v.APIKey, NotifyURL: v.NotifyURL, RefundNotifyURL: v.RefundNotifyURL}
	if c.AppID == "" || c.MerchantID == "" || c.Serial == "" || c.PublicKeyID == "" || len(c.APIKey) != 32 || !validNotifyURL(c.NotifyURL) || !validNotifyURL(c.RefundNotifyURL) {
		return c, fail(503, "微信支付配置不完整，未产生扣款")
	}
	if err := validatePaymentDraft(v); err != nil {
		return c, fail(503, "微信支付配置无效，请联系管理员")
	}
	var err error
	c.PrivateKey, err = parseMerchantKey(v.PrivatePEM)
	if err != nil {
		return c, fail(503, "支付商户私钥未正确配置")
	}
	c.PublicKey, err = parsePlatformKey(v.PublicPEM)
	if err != nil {
		return c, fail(503, "微信支付验签公钥未正确配置")
	}
	return c, nil
}

// paymentSettingsFromEnvironment 兼容既有环境变量部署，文件内容仅留在服务器内存。
func paymentSettingsFromEnvironment() paymentSettings {
	private, _ := os.ReadFile(os.Getenv("WECHAT_PAY_PRIVATE_KEY_PATH"))
	public, _ := os.ReadFile(os.Getenv("WECHAT_PAY_PUBLIC_KEY_PATH"))
	return paymentSettings{MerchantID: os.Getenv("WECHAT_PAY_MCH_ID"), CertificateSerial: os.Getenv("WECHAT_PAY_SERIAL"), PublicKeyID: os.Getenv("WECHAT_PAY_PUBLIC_KEY_ID"), NotifyURL: os.Getenv("WECHAT_PAY_NOTIFY_URL"), RefundNotifyURL: os.Getenv("WECHAT_PAY_REFUND_NOTIFY_URL"), APIKey: os.Getenv("WECHAT_PAY_API_V3_KEY"), PrivatePEM: string(private), PublicPEM: string(public)}
}

// loadPaymentSettings 在启动阶段加载一次加密配置，交易持锁期间绝不另取数据库连接。
func (a *App) loadPaymentSettings(ctx context.Context) error {
	encrypted, err := get[encryptedPaymentSettings](ctx, a.DB, "settings", "payment-private", false)
	if missing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	value, err := openPaymentSettings(encrypted)
	if err != nil {
		return err
	}
	runtime, readyErr := a.runtimePaymentConfig(value)
	a.paymentConfigMu.Lock()
	a.paymentConfig = &paymentConfigState{Settings: value, Runtime: runtime, Ready: readyErr == nil}
	a.paymentConfigMu.Unlock()
	return nil
}

// paymentSettingsView 只返回运营信息和秘密是否已配置，任何响应都不会包含原始秘密。
func (a *App) paymentSettingsView(value paymentSettings, source string) map[string]any {
	_, readyErr := a.runtimePaymentConfig(value)
	_, storageErr := paymentCipher()
	return map[string]any{"merchant_id": value.MerchantID, "certificate_serial": value.CertificateSerial, "platform_public_key_id": value.PublicKeyID, "notify_url": value.NotifyURL, "refund_notify_url": value.RefundNotifyURL, "api_v3_key_set": value.APIKey != "", "merchant_private_key_set": value.PrivatePEM != "", "platform_public_key_set": value.PublicPEM != "", "configured": readyErr == nil, "source": source, "storage_available": storageErr == nil}
}

// adminPaymentSettings 向管理员显示脱敏配置状态，其他身份无权查看商户参数。
func (a *App) adminPaymentSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	a.paymentConfigMu.RLock()
	snapshot := a.paymentConfig
	a.paymentConfigMu.RUnlock()
	if snapshot != nil {
		return a.paymentSettingsView(snapshot.Settings, "admin"), nil
	}
	return a.paymentSettingsView(paymentSettingsFromEnvironment(), "environment"), nil
}

// savePaymentSettings 串行合并草稿、加密落库并原子发布快照，空秘密输入表示保留原值。
func (a *App) savePaymentSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var patch struct {
		MerchantID        *string `json:"merchant_id"`
		CertificateSerial *string `json:"certificate_serial"`
		PublicKeyID       *string `json:"platform_public_key_id"`
		NotifyURL         *string `json:"notify_url"`
		RefundNotifyURL   *string `json:"refund_notify_url"`
		APIKey            string  `json:"api_v3_key"`
		PrivatePEM        string  `json:"merchant_private_key_pem"`
		PublicPEM         string  `json:"platform_public_key_pem"`
	}
	if err = decode(r, &patch); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(8173021)"); err != nil {
		return nil, err
	}
	previous := paymentSettingsFromEnvironment()
	encrypted, err := get[encryptedPaymentSettings](r.Context(), tx, "settings", "payment-private", false)
	if err == nil {
		previous, err = openPaymentSettings(encrypted)
	} else if missing(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	in := previous
	for _, field := range []struct {
		input  *string
		target *string
	}{{patch.MerchantID, &in.MerchantID}, {patch.CertificateSerial, &in.CertificateSerial}, {patch.PublicKeyID, &in.PublicKeyID}, {patch.NotifyURL, &in.NotifyURL}, {patch.RefundNotifyURL, &in.RefundNotifyURL}} {
		if field.input != nil {
			*field.target = strings.TrimSpace(*field.input)
		}
	}
	if patch.APIKey != "" {
		in.APIKey = patch.APIKey
	}
	if strings.TrimSpace(patch.PrivatePEM) != "" {
		in.PrivatePEM = patch.PrivatePEM
	}
	if strings.TrimSpace(patch.PublicPEM) != "" {
		in.PublicPEM = patch.PublicPEM
	}
	if err = validatePaymentDraft(in); err != nil {
		return nil, err
	}
	encrypted, err = sealPaymentSettings(in)
	if err != nil {
		return nil, err
	}
	if err = put(r.Context(), tx, "settings", "payment-private", encrypted); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "payment.settings.save", "payment-private", "更新加密商户配置"); err != nil {
		return nil, err
	}
	runtime, readyErr := a.runtimePaymentConfig(in)
	// 只在已取得数据库连接且提交时持快照锁，避免与持有订单事务的支付读取互相等待。
	a.paymentConfigMu.Lock()
	defer a.paymentConfigMu.Unlock()
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	a.paymentConfig = &paymentConfigState{Settings: in, Runtime: runtime, Ready: readyErr == nil}
	return a.paymentSettingsView(in, "admin"), nil
}

// registerPaymentSettingsRoutes 单独注册支付配置端点，避免通用管理列表意外暴露秘密。
func (a *App) registerPaymentSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/payment-settings", a.wrap(a.adminPaymentSettings))
	mux.HandleFunc("PATCH /api/admin/payment-settings", a.wrap(a.savePaymentSettings))
}
