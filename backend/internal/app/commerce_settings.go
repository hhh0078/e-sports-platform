// Package app 管理双微信入口、虚拟支付和分终端手续费，秘密加密保存且不进入公开响应。
package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// commerceSettings 保存渠道配置；手续费单位为万分之一，金币兑换固定一元一金币。
type commerceSettings struct {
	Mode                   string   `json:"mode"`
	FeeBearer              string   `json:"fee_bearer"`
	AndroidFeeBPS          int64    `json:"android_fee_bps"`
	IOSFeeBPS              int64    `json:"ios_fee_bps"`
	OfferID                string   `json:"offer_id"`
	AppKey                 string   `json:"app_key"`
	SandboxAppKey          string   `json:"sandbox_app_key"`
	VirtualEnv             int      `json:"virtual_env"`
	OfficialAppID          string   `json:"official_app_id"`
	OfficialSecret         string   `json:"official_secret"`
	OfficialRedirectURL    string   `json:"official_redirect_url"`
	PreviousOfficialAppIDs []string `json:"previous_official_app_ids,omitempty"`
}

// environmentCommerceSettings 读取服务端部署配置，缺省使用小程序虚拟支付和平台承担手续费。
func environmentCommerceSettings() commerceSettings {
	sandbox, _ := strconv.Atoi(os.Getenv("WECHAT_VIRTUAL_ENV"))
	return commerceSettings{Mode: env("WECHAT_COMMERCE_MODE", "mini_program"), FeeBearer: "platform", OfferID: os.Getenv("WECHAT_VIRTUAL_OFFER_ID"), AppKey: os.Getenv("WECHAT_VIRTUAL_APP_KEY"), SandboxAppKey: os.Getenv("WECHAT_VIRTUAL_SANDBOX_APP_KEY"), VirtualEnv: sandbox, OfficialAppID: os.Getenv("WECHAT_OFFICIAL_APP_ID"), OfficialSecret: os.Getenv("WECHAT_OFFICIAL_APP_SECRET"), OfficialRedirectURL: os.Getenv("WECHAT_OFFICIAL_REDIRECT_URL")}
}

// commerceSettings 返回只读配置副本，不在支付事务中另取数据库连接。
func (a *App) commerceSettings() commerceSettings {
	a.paymentConfigMu.RLock()
	defer a.paymentConfigMu.RUnlock()
	if a.commerceConfig != nil {
		value := *a.commerceConfig
		value.PreviousOfficialAppIDs = append([]string(nil), value.PreviousOfficialAppIDs...)
		return value
	}
	return environmentCommerceSettings()
}

// validateCommerceSettings 校验显式渠道和费率边界，允许未填秘密的配置草稿。
func validateCommerceSettings(v commerceSettings) error {
	if v.Mode != "mini_program" && v.Mode != "official_account" {
		return fail(400, "请选择小程序模式或公众号模式")
	}
	if v.FeeBearer != "platform" && v.FeeBearer != "buyer" {
		return fail(400, "请选择平台承担或买家承担手续费")
	}
	if v.AndroidFeeBPS < 0 || v.AndroidFeeBPS >= 10000 || v.IOSFeeBPS < 0 || v.IOSFeeBPS >= 10000 {
		return fail(400, "手续费比例需在0%至100%之间且小于100%")
	}
	if v.VirtualEnv != 0 && v.VirtualEnv != 1 {
		return fail(400, "虚拟支付环境只能为正式或沙箱")
	}
	if v.OfferID != "" && !regexp.MustCompile(`^[0-9]{1,32}$`).MatchString(v.OfferID) {
		return fail(400, "虚拟支付OfferID必须为数字")
	}
	if v.OfficialAppID != "" && !regexp.MustCompile(`^wx[0-9a-zA-Z]{16}$`).MatchString(v.OfficialAppID) {
		return fail(400, "公众号AppID格式无效")
	}
	for _, key := range []string{v.AppKey, v.SandboxAppKey, v.OfficialSecret} {
		if key != "" && (len(key) < 16 || len(key) > 256 || strings.ContainsAny(key, "\r\n\t ")) {
			return fail(400, "微信密钥长度或格式无效")
		}
	}
	if v.OfficialRedirectURL != "" && (len(v.OfficialRedirectURL) > 2048 || !validNotifyURL(v.OfficialRedirectURL)) {
		return fail(400, "公众号授权回跳地址必须为有效HTTPS地址")
	}
	return nil
}

// sealCommerceSettings 使用独立用途标签加密整份渠道配置，防止密文跨领域替换。
func sealCommerceSettings(v commerceSettings) (encryptedPaymentSettings, error) {
	gcm, err := paymentCipher()
	if err != nil {
		return encryptedPaymentSettings{}, err
	}
	plain, err := json.Marshal(v)
	if err != nil {
		return encryptedPaymentSettings{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedPaymentSettings{}, err
	}
	sealed := gcm.Seal(nonce, nonce, plain, []byte("jijie-commerce-settings-v1"))
	return encryptedPaymentSettings{Version: 1, Ciphertext: base64.StdEncoding.EncodeToString(sealed)}, nil
}

// openCommerceSettings 校验密文和配置边界，解密失败时保持支付关闭。
func openCommerceSettings(v encryptedPaymentSettings) (commerceSettings, error) {
	var result commerceSettings
	gcm, err := paymentCipher()
	if err != nil {
		return result, err
	}
	raw, err := base64.StdEncoding.DecodeString(v.Ciphertext)
	if err != nil || v.Version != 1 || len(raw) < gcm.NonceSize() {
		return result, fail(503, "微信渠道配置密文无效")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte("jijie-commerce-settings-v1"))
	if err != nil || json.Unmarshal(plain, &result) != nil {
		return result, fail(503, "微信渠道配置无法解密，请核对服务器加密配置")
	}
	if err = validateCommerceSettings(result); err != nil {
		return result, fail(503, "微信渠道配置无效，请联系平台管理员")
	}
	return result, nil
}

// loadCommerceSettings 启动时恢复渠道快照；没有保存记录时继续使用部署环境配置。
func (a *App) loadCommerceSettings(ctx context.Context) error {
	encrypted, err := get[encryptedPaymentSettings](ctx, a.DB, "settings", "commerce-private", false)
	if missing(err) {
		return validateCommerceSettings(environmentCommerceSettings())
	}
	if err != nil {
		return err
	}
	value, err := openCommerceSettings(encrypted)
	if err != nil {
		return err
	}
	a.paymentConfigMu.Lock()
	a.commerceConfig = &value
	a.paymentConfigMu.Unlock()
	return nil
}

// commercePublicView 仅给登录与付款界面展示公开能力，不下发OfferID、AppID和任何秘密。
func (a *App) commercePublicView() map[string]any {
	v := a.commerceSettings()
	virtualKey := v.AppKey
	if v.VirtualEnv == 1 {
		virtualKey = v.SandboxAppKey
	}
	_, smsErr := readPhoneSMSConfig()
	return map[string]any{"mode": v.Mode, "fee_bearer": v.FeeBearer, "android_fee_bps": v.AndroidFeeBPS, "ios_fee_bps": v.IOSFeeBPS, "coin_rate": 1, "virtual_configured": a.Config.WechatAppID != "" && a.Config.WechatMessageToken != "" && v.OfferID != "" && virtualKey != "", "callback_configured": a.Config.WechatMessageToken != "", "official_configured": v.OfficialAppID != "" && v.OfficialSecret != "" && v.OfficialRedirectURL != "", "sms_configured": smsErr == nil}
}

// commerceSettingsView 返回管理员可见的配置和秘密存在标记，禁止序列化内部原始配置。
func (a *App) commerceSettingsView(v commerceSettings) map[string]any {
	_, storageErr := paymentCipher()
	key := v.AppKey
	if v.VirtualEnv == 1 {
		key = v.SandboxAppKey
	}
	_, smsErr := readPhoneSMSConfig()
	return map[string]any{"mode": v.Mode, "fee_bearer": v.FeeBearer, "android_fee_bps": v.AndroidFeeBPS, "ios_fee_bps": v.IOSFeeBPS, "coin_rate": 1, "offer_id": v.OfferID, "virtual_env": v.VirtualEnv, "official_app_id": v.OfficialAppID, "official_redirect_url": v.OfficialRedirectURL, "app_key_set": v.AppKey != "", "sandbox_app_key_set": v.SandboxAppKey != "", "official_secret_set": v.OfficialSecret != "", "virtual_configured": a.Config.WechatAppID != "" && a.Config.WechatMessageToken != "" && v.OfferID != "" && key != "", "callback_configured": a.Config.WechatMessageToken != "", "official_configured": v.OfficialAppID != "" && v.OfficialSecret != "" && v.OfficialRedirectURL != "", "sms_configured": smsErr == nil, "storage_available": storageErr == nil}
}

// adminCommerceSettings 只允许当前平台管理员读取脱敏渠道配置。
func (a *App) adminCommerceSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	return a.commerceSettingsView(a.commerceSettings()), nil
}

// saveCommerceSettings 原子保存配置及审计，空秘密输入保留既有值，历史公众号AppID保留供旧单验证。
func (a *App) saveCommerceSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var patch struct {
		Mode                *string `json:"mode"`
		FeeBearer           *string `json:"fee_bearer"`
		AndroidFeeBPS       *int64  `json:"android_fee_bps"`
		IOSFeeBPS           *int64  `json:"ios_fee_bps"`
		OfferID             *string `json:"offer_id"`
		VirtualEnv          *int    `json:"virtual_env"`
		OfficialAppID       *string `json:"official_app_id"`
		OfficialRedirectURL *string `json:"official_redirect_url"`
		AppKey              string  `json:"app_key"`
		SandboxAppKey       string  `json:"sandbox_app_key"`
		OfficialSecret      string  `json:"official_secret"`
	}
	if err = decode(r, &patch); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(8173023)"); err != nil {
		return nil, err
	}
	in := environmentCommerceSettings()
	stored, err := get[encryptedPaymentSettings](r.Context(), tx, "settings", "commerce-private", false)
	if err == nil {
		in, err = openCommerceSettings(stored)
	} else if missing(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	previousAppID := in.OfficialAppID
	for _, field := range []struct {
		input  *string
		target *string
	}{{patch.Mode, &in.Mode}, {patch.FeeBearer, &in.FeeBearer}, {patch.OfferID, &in.OfferID}, {patch.OfficialAppID, &in.OfficialAppID}, {patch.OfficialRedirectURL, &in.OfficialRedirectURL}} {
		if field.input != nil {
			*field.target = strings.TrimSpace(*field.input)
		}
	}
	if patch.AndroidFeeBPS != nil {
		in.AndroidFeeBPS = *patch.AndroidFeeBPS
	}
	if patch.IOSFeeBPS != nil {
		in.IOSFeeBPS = *patch.IOSFeeBPS
	}
	if patch.VirtualEnv != nil {
		in.VirtualEnv = *patch.VirtualEnv
	}
	for _, field := range []struct {
		input  string
		target *string
	}{{patch.AppKey, &in.AppKey}, {patch.SandboxAppKey, &in.SandboxAppKey}, {patch.OfficialSecret, &in.OfficialSecret}} {
		if strings.TrimSpace(field.input) != "" {
			*field.target = strings.TrimSpace(field.input)
		}
	}
	if previousAppID != "" && previousAppID != in.OfficialAppID && !contains(in.PreviousOfficialAppIDs, previousAppID) {
		in.PreviousOfficialAppIDs = append(in.PreviousOfficialAppIDs, previousAppID)
	}
	if err = validateCommerceSettings(in); err != nil {
		return nil, err
	}
	encrypted, err := sealCommerceSettings(in)
	if err != nil {
		return nil, err
	}
	if err = put(r.Context(), tx, "settings", "commerce-private", encrypted); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "commerce.settings.save", "commerce-private", "更新微信入口、虚拟支付及手续费配置"); err != nil {
		return nil, err
	}
	a.paymentConfigMu.Lock()
	defer a.paymentConfigMu.Unlock()
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	a.commerceConfig = &in
	return a.commerceSettingsView(in), nil
}

// registerCommerceSettingsRoutes 注册独立加密配置入口，不经过通用管理列表。
func (a *App) registerCommerceSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/commerce-settings", a.wrap(a.adminCommerceSettings))
	mux.HandleFunc("PATCH /api/admin/commerce-settings", a.wrap(a.saveCommerceSettings))
}

// loadWXPayForApp 按交易原始AppID恢复普通微信配置，防止模式切换改变旧单支付归属。
func (a *App) loadWXPayForApp(appID string) (wxPayConfig, error) {
	if appID == "" {
		return a.loadWXPay()
	}
	commerce := a.commerceSettings()
	if appID != a.Config.WechatAppID && appID != commerce.OfficialAppID && !contains(commerce.PreviousOfficialAppIDs, appID) {
		return wxPayConfig{}, fail(409, "原交易微信身份配置无法匹配，请联系平台核查")
	}
	a.paymentConfigMu.RLock()
	snapshot := a.paymentConfig
	a.paymentConfigMu.RUnlock()
	var c wxPayConfig
	var err error
	if snapshot != nil {
		if !snapshot.Ready {
			return c, fail(503, "微信支付配置不完整，未产生扣款")
		}
		c = snapshot.Runtime
	} else {
		c, err = a.runtimePaymentConfig(paymentSettingsFromEnvironment())
		if err != nil {
			return c, err
		}
	}
	c.AppID = appID
	return c, nil
}
