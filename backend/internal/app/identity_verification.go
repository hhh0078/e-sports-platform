// Package app 执行成年人姓名身份证核验，仅持久化必要脱敏结果并以私密事实保护付款门槛。
package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// IdentityStatus 是仅返回给本人使用的身份核验状态，不包含姓名及完整身份证。
type IdentityStatus struct {
	IsMinor    bool   `json:"is_minor"`
	Verified   bool   `json:"verified"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

// identityStatus 从用户表和私密通过事实实时读取状态，旧数据或缺失事实一律不能付款。
func identityStatus(ctx context.Context, q querier, userID string) (IdentityStatus, error) {
	u, err := get[User](ctx, q, "users", userID, false)
	if err != nil {
		return IdentityStatus{IsMinor: true}, err
	}
	identity, err := get[map[string]string](ctx, q, "identities", userID, false)
	if err != nil && !missing(err) {
		return IdentityStatus{IsMinor: true}, err
	}
	status := IdentityStatus{IsMinor: u.IsMinor}
	_, timestampErr := time.Parse(time.RFC3339Nano, identity["identity_verified_at"])
	status.Verified = !u.IsMinor && u.Status == "active" && identity["identity_source"] == "xxapi" && identity["identity_adult"] == "true" && timestampErr == nil
	if status.Verified {
		status.VerifiedAt = identity["identity_verified_at"]
	}
	return status, nil
}

// requireAdultIdentity 在付款事务使用当前数据库记录核验成年资格，不信任请求或缓存布尔值。
func requireAdultIdentity(ctx context.Context, q querier, userID string) error {
	status, err := identityStatus(ctx, q, userID)
	if err != nil {
		return err
	}
	if !status.Verified {
		return &apiError{Status: 428, Code: "IDENTITY_REQUIRED", Message: "请先完成身份核验，确认年满18周岁后再下单支付"}
	}
	return nil
}

// myIdentityStatus 仅向当前登录本人返回实时核验状态。
func (a *App) myIdentityStatus(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	return identityStatus(r.Context(), a.DB, u.ID)
}

// verifyIdentity 先本地拒绝未成年人，再向固定提供方请求二要素核验，通过后原子更新用户及私密事实。
func (a *App) verifyIdentity(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Name    string `json:"name"`
		IDCard  string `json:"idcard"`
		Consent bool   `json:"consent"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !in.Consent {
		return nil, fail(400, "请先同意为身份核验提交姓名和身份证信息")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.IDCard = strings.ToUpper(strings.TrimSpace(in.IDCard))
	if err = validateIdentityName(in.Name); err != nil {
		return nil, err
	}
	adult, err := identityAdult(in.IDCard, time.Now())
	if err != nil {
		return nil, err
	}
	if !adult {
		return nil, &apiError{Status: 403, Code: "IDENTITY_UNDERAGE", Message: "您未满18岁，我们无法为您服务，请退出该平台。"}
	}
	current, err := identityStatus(r.Context(), a.DB, u.ID)
	if err != nil || current.Verified {
		return current, err
	}
	if err = a.throttle(r.Context(), "identity-verify", u.ID, 3); err != nil {
		return nil, err
	}
	// 单用户短租约避免并发点击产生重复收费；随机持有者只删除自己的租约。
	leaseKey, leaseValue := a.redisKey("identity-verifying:"+u.ID), newID()
	acquired, err := a.Redis.SetNX(r.Context(), leaseKey, leaseValue, 15*time.Second).Result()
	if err != nil {
		return nil, fail(503, "核验服务暂时不可用，请稍后重试")
	}
	if !acquired {
		return nil, fail(429, "正在核验身份，请勿重复提交")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
		defer cancel()
		a.Redis.Eval(ctx, `if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end; return 0`, []string{leaseKey}, leaseValue)
	}()
	// 另一次核验可能在读取状态与取得租约之间刚刚完成，再读一次避免重复付费。
	current, err = identityStatus(r.Context(), a.DB, u.ID)
	if err != nil || current.Verified {
		return current, err
	}
	settings, err := readIdentitySettings(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	if settings.APIKey == "" {
		return nil, fail(503, "身份核验服务尚未配置，请联系平台管理员")
	}
	requestID, err := a.callIdentityCheck(r.Context(), settings.APIKey, in.Name, in.IDCard)
	if err != nil {
		return nil, err
	}
	return a.storeAdultIdentity(r.Context(), u.ID, in.Name, in.IDCard, requestID)
}

// callIdentityCheck 发送限时表单请求，拒绝重定向，仅明确一致才返回请求摘要。
func (a *App) callIdentityCheck(ctx context.Context, apiKey, name, card string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	form := url.Values{"name": {name}, "idcard": {card}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, identityAPIURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fail(503, "身份核验请求暂时不可用")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{Timeout: 8 * time.Second}
	if a.HTTPClient != nil {
		client = *a.HTTPClient
		client.Timeout = 8 * time.Second
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return "", fail(502, "身份核验服务请求失败或超时，请稍后重试")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32769))
	if err != nil || len(raw) > 32768 {
		return "", fail(502, "身份核验服务返回异常，请稍后重试")
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Result bool   `json:"result"`
			Desc   string `json:"desc"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return "", fail(502, "身份核验服务返回异常，请稍后重试")
	}
	if response.StatusCode != 200 || result.Code != 200 {
		return "", fail(502, identityProviderMessage(result.Msg, "身份核验失败，请稍后重试", apiKey, name, card))
	}
	if !result.Data.Result {
		return "", fail(422, identityProviderMessage(result.Data.Desc, "姓名与身份证号不一致，请核对后重新提交", apiKey, name, card))
	}
	if !identityRequestPattern.MatchString(result.RequestID) {
		return "", fail(502, "身份核验服务返回异常，请稍后重试")
	}
	return result.RequestID, nil
}

// identityProviderMessage 保留提供方可读提示，同时避免上游错误回显姓名、证件或密钥。
func identityProviderMessage(message, fallback string, secrets ...string) string {
	message = strings.TrimSpace(message)
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "***")
		}
	}
	if !length(message, 1, 240) {
		return fallback
	}
	return message
}

// storeAdultIdentity 使用用户行锁合并私密核验事实，完整姓名和证件号均不写入数据库。
func (a *App) storeAdultIdentity(ctx context.Context, userID, name, card, requestID string) (IdentityStatus, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return IdentityStatus{}, err
	}
	defer tx.Rollback(ctx)
	u, err := get[User](ctx, tx, "users", userID, true)
	if err != nil {
		return IdentityStatus{}, err
	}
	if u.Status != "active" {
		return IdentityStatus{}, fail(403, "账号已被停用")
	}
	verifiedAt := now()
	fields := map[string]string{"identity_verified_at": verifiedAt, "identity_source": "xxapi", "identity_adult": "true", "identity_request_id": requestID, "identity_name_masked": string([]rune(name)[0]) + "*", "identity_card_masked": "**************" + card[14:], "identity_consent_at": verifiedAt}
	if err = mergeIdentity(ctx, tx, userID, fields); err != nil {
		return IdentityStatus{}, err
	}
	u.IsMinor = false
	if err = put(ctx, tx, "users", userID, u); err != nil {
		return IdentityStatus{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return IdentityStatus{}, err
	}
	return IdentityStatus{IsMinor: false, Verified: true, VerifiedAt: verifiedAt}, nil
}

// registerIdentityVerificationRoutes 独立挂载本人核验与管理员密钥配置，避免通用资料接口修改核验标记。
func (a *App) registerIdentityVerificationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/me/identity", a.wrap(a.myIdentityStatus))
	mux.HandleFunc("POST /api/me/identity/verify", a.wrap(a.verifyIdentity))
	mux.HandleFunc("GET /api/admin/identity-settings", a.wrap(a.adminIdentitySettings))
	mux.HandleFunc("PUT /api/admin/identity-settings", a.wrap(a.saveIdentitySettings))
}
