// Package app 提供身份证核验密钥的管理员加密配置，接口地址固定且任何响应均不回显密钥。
package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// identitySettings 为仅供服务端解密使用的姓名身份证二要素接口配置。
type identitySettings struct {
	APIKey string `json:"api_key"`
}

// sealIdentitySettings 使用随机 nonce 和独立用途标签加密核验密钥。
func sealIdentitySettings(value identitySettings) (encryptedPaymentSettings, error) {
	gcm, err := paymentCipher()
	if err != nil {
		return encryptedPaymentSettings{}, fail(503, "服务器尚未配置资料加密主密钥 CONFIG_ENCRYPTION_KEY")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return encryptedPaymentSettings{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedPaymentSettings{}, err
	}
	return encryptedPaymentSettings{Version: 1, Ciphertext: base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, raw, []byte("jijie-identity-settings-v1")))}, nil
}

// openIdentitySettings 验证密文完整性及用途，错误主密钥或其他配置密文均不能解开。
func openIdentitySettings(value encryptedPaymentSettings) (identitySettings, error) {
	var out identitySettings
	gcm, err := paymentCipher()
	if err != nil {
		return out, err
	}
	raw, err := base64.StdEncoding.DecodeString(value.Ciphertext)
	if err != nil || value.Version != 1 || len(raw) < gcm.NonceSize() {
		return out, fail(503, "身份核验配置密文格式无效")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte("jijie-identity-settings-v1"))
	if err != nil || json.Unmarshal(plain, &out) != nil {
		return out, fail(503, "身份核验配置无法解密，请联系管理员")
	}
	return out, nil
}

// readIdentitySettings 读取独立的加密配置记录，未配置时返回空密钥。
func readIdentitySettings(ctx context.Context, q querier) (identitySettings, error) {
	v, err := get[encryptedPaymentSettings](ctx, q, "settings", "identity-private", false)
	if missing(err) {
		return identitySettings{}, nil
	}
	if err != nil {
		return identitySettings{}, err
	}
	return openIdentitySettings(v)
}

// identitySettingsView 输出模式和密钥状态；解密异常不阻止管理员选择独立的基础校验。
func identitySettingsView(v identitySettings, policy identityPolicy, keyError error) map[string]any {
	_, cipherErr := paymentCipher()
	view := map[string]any{"mode": policy.Mode, "has_api_key": v.APIKey != "", "configured": policy.Mode == identityBasic || v.APIKey != "" && cipherErr == nil && keyError == nil, "storage_available": cipherErr == nil, "api_url": identityAPIURL}
	if keyError != nil {
		view["api_error"] = "原接口密钥暂不可读取；基础校验仍可使用，严格核验前请检查服务端加密配置"
	}
	return view
}

// adminIdentitySettings 仅平台管理员读取模式和脱敏状态，不向客户端回显秘密。
func (a *App) adminIdentitySettings(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	policy, err := readIdentityPolicy(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	v, keyErr := readIdentitySettings(r.Context(), a.DB)
	return identitySettingsView(v, policy, keyErr), nil
}

// saveIdentitySettings 串行保存规则；空密钥不读取或覆盖旧密文，基础校验不依赖第三方配置。
func (a *App) saveIdentitySettings(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		APIKey string `json:"api_key"`
		Mode   string `json:"mode"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if len(in.APIKey) > 4096 || strings.ContainsAny(in.APIKey, "\r\n") {
		return nil, fail(400, "核验密钥格式无效")
	}
	if in.Mode != "" && in.Mode != identityStrict && in.Mode != identityBasic {
		return nil, fail(400, "请选择有效的身份校验方式")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		return nil, err
	}
	policy, err := readIdentityPolicy(r.Context(), tx)
	if err != nil {
		return nil, err
	}
	if in.Mode != "" {
		policy.Mode = in.Mode
	}
	if strings.TrimSpace(in.APIKey) != "" {
		encrypted, sealErr := sealIdentitySettings(identitySettings{APIKey: strings.TrimSpace(in.APIKey)})
		if sealErr != nil {
			return nil, sealErr
		}
		if err = put(r.Context(), tx, "settings", "identity-private", encrypted); err != nil {
			return nil, err
		}
	}
	if err = put(r.Context(), tx, "settings", "identity-policy", policy); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "identity.settings.save", "identity-policy", "保存身份校验方式："+policy.Mode); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	v, keyErr := readIdentitySettings(r.Context(), a.DB)
	return identitySettingsView(v, policy, keyErr), nil
}
