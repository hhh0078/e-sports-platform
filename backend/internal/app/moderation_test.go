// Package app 验证AI协议、配置加密、审核失败关闭及各个用户内容入口不可绕过。
package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// installMockAI 配置真正经过HTTP协议解析的模型替身，与微信支付传输层独立。
func installMockAI(t *testing.T, api *testAPI, handler func(*http.Request) (*http.Response, error)) {
	t.Helper()
	t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32))))
	api.call("admin", "PATCH", "/admin/ai-settings", map[string]string{"api_key": "ai-test-secret-not-public"}, 200, nil)
	api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(handler)}
}

// aiMockResponse 构造OpenAI兼容响应，不绕过服务端的严格JSON验证。
func aiMockResponse(content string) *http.Response {
	raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}, "finish_reason": "stop"}}})
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw)))}
}

// mockAllowAI 为其它生产边界集成测试提供明确的合规服务替身，不能用于生产放行。
func mockAllowAI(t *testing.T, api *testAPI) {
	installMockAI(t, api, func(*http.Request) (*http.Response, error) {
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})
}

// TestAIAddressAndDecisionBoundary 验证公网目标限制、DNS地址分类和严格审核结果。
func TestAIAddressAndDecisionBoundary(t *testing.T) {
	for _, address := range []string{"http://api.example.com/v1", "https://localhost/v1", "https://127.0.0.1/v1", "https://[::1]/v1", "https://10.2.0.1/v1", "https://100.100.100.200/v1", "https://192.0.2.1/v1", "https://u:pass@api.example.com/v1", "https://api.example.com:8443/v1", "https://api.example.com/v1?key=secret", "https://api.example.com/v1#x", "https://foo.internal/v1", "https://[64:ff9b::a00:1]/v1", "https://[::ffff:0:a00:1]/v1"} {
		t.Run(address, func(t *testing.T) {
			v := defaultAISettings()
			v.BaseURL = address
			if validateAISettings(v) == nil {
				t.Fatal("不安全审核端点被接受")
			}
		})
	}
	if validateAISettings(defaultAISettings()) != nil {
		t.Fatal("默认公网HTTPS地址被拒绝")
	}
	for _, raw := range []string{`{"decision":"allow"}`, `{"decision":"reject","reason":"内容涉嫌违法交易"}`} {
		if _, err := parseModerationDecision(raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{"decision":"unknown"}`, `{"decision":"reject","reason":""}`, `{"decision":"allow","ok":true}`, "```json\n{}\n```", `{"decision":"allow"} {}`, `null`} {
		if _, err := parseModerationDecision(raw); err == nil {
			t.Fatalf("无效结果被放行：%s", raw)
		}
	}
	client := newAIHTTPClient()
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if client.CheckRedirect(req, nil) != http.ErrUseLastResponse {
		t.Fatal("审核密钥可能跟随重定向")
	}
}

// TestIntegrationAIConfigPrivacyAndFailClosed 验证五类权限、加密保存、默认值和真实连接不使用模拟。
func TestIntegrationAIConfigPrivacyAndFailClosed(t *testing.T) {
	api := newTestAPI(t)
	for _, user := range []string{"", "player", "worker", "clubowner", "outsider"} {
		want := 403
		if user == "" {
			want = 401
		}
		api.call(user, "PATCH", "/admin/ai-settings", map[string]string{"api_key": "must-not-save"}, want, nil)
		for _, path := range []string{"/admin/ai-settings", "/admin/ai-settings/test"} {
			method := "GET"
			if strings.HasSuffix(path, "test") {
				method = "POST"
			}
			api.call(user, method, path, map[string]any{}, want, nil)
		}
	}
	var view struct {
		BaseURL, Model, Mode string
		HasAPIKey            bool `json:"has_api_key"`
	}
	api.call("admin", "GET", "/admin/ai-settings", nil, 200, &view)
	if view.Model != defaultAIModel || view.Mode != "local_simulation" || view.HasAPIKey {
		t.Fatalf("默认审核配置错误：%+v", view)
	}
	api.call("admin", "POST", "/admin/ai-settings/test", map[string]any{}, 503, nil)
	api.App.Config.DevMode = false
	code, raw, err := api.request("worker", "PATCH", "/me/profile", map[string]string{"bio": "正式环境没有密钥必须暂停提交"})
	if err != nil || code != 503 || !strings.Contains(string(raw), "moderation_unavailable") {
		t.Fatalf("缺密钥被放行：%d %s %v", code, raw, err)
	}
	mockAllowAI(t, api)
	api.call("admin", "POST", "/admin/ai-settings/test", map[string]any{}, 200, nil)
	for _, path := range []string{"/admin/ai-settings", "/admin/settings", "/bootstrap", "/admin/audit"} {
		code, raw, err = api.request("admin", "GET", path, nil)
		if err != nil || code != 200 || strings.Contains(string(raw), "ai-test-secret-not-public") {
			t.Fatalf("AI秘密从%s泄露：%d", path, code)
		}
	}
	encrypted, err := get[encryptedPaymentSettings](t.Context(), api.App.DB, "settings", "ai-private", false)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if strings.Contains(string(decoded), "ai-test-secret-not-public") {
		t.Fatal("数据库保存了明文密钥")
	}
	api.call("admin", "PATCH", "/admin/ai-settings", map[string]string{"api_key": "", "model": "new-model"}, 200, nil)
	saved, err := api.App.readAISettings(t.Context(), api.App.DB)
	if err != nil || saved.APIKey != "ai-test-secret-not-public" || saved.Model != "new-model" {
		t.Fatal("空密钥覆盖了既有配置")
	}
	api.call("admin", "PATCH", "/admin/ai-settings", map[string]any{"clear_api_key": true, "api_key": "contradiction"}, 400, nil)
	api.call("admin", "PATCH", "/admin/ai-settings", map[string]bool{"clear_api_key": true}, 200, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "密钥清除后应立即停止发布"}, 503, nil)
}

// TestIntegrationAIHTTPProtocolAndImageFailure 验证多模态真实字节、无认证秘密、无效响应与网络错误均不写入。
func TestIntegrationAIHTTPProtocolAndImageFailure(t *testing.T) {
	api := newTestAPI(t)
	var calls atomic.Int32
	var last string
	installMockAI(t, api, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		last = string(raw)
		if r.Method != "POST" || r.URL.String() != defaultAIBaseURL+"/chat/completions" || r.Header.Get("Authorization") != "Bearer ai-test-secret-not-public" {
			t.Error("AI协议请求错误")
		}
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "正常游戏任务服务介绍"}, 200, nil)
	if !strings.Contains(last, "正常游戏任务服务介绍") || strings.Contains(last, "openid") || strings.Contains(last, "phone_number") || strings.Contains(last, "ai-test-secret-not-public") {
		t.Fatal("审核载荷缺少内容或包含无关秘密")
	}
	o := api.newOrder()
	api.uploadTestImage("player", o.ID)
	if !strings.Contains(last, "data:image/png;base64,") || strings.Contains(last, api.Server.URL) {
		t.Fatal("图片审核未发送真实字节或错误使用外部URL")
	}
	before := calls.Load()
	api.call("outsider", "POST", "/orders/"+o.ID+"/messages", map[string]string{"content": "无权阅读的订单不能进入外部审核"}, 403, nil)
	if calls.Load() != before {
		t.Fatal("未授权内容发给了模型")
	}
	for _, scenario := range []string{"network", "malformed", "length", "upstream"} {
		t.Run(scenario, func(t *testing.T) {
			api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(func(*http.Request) (*http.Response, error) {
				switch scenario {
				case "network":
					return nil, context.DeadlineExceeded
				case "upstream":
					return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("ai-test-secret-not-public"))}, nil
				case "length":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 65537)))}, nil
				}
				return aiMockResponse("not valid json"), nil
			})}
			code, raw, err := api.request("worker", "PATCH", "/me/profile", map[string]string{"bio": "失败时不能保存的新简介"})
			if err != nil || code != 503 || strings.Contains(string(raw), "ai-test-secret-not-public") {
				t.Fatalf("审核故障未关闭或泄密：%d %s", code, raw)
			}
			saved, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
			if saved.Bio == "失败时不能保存的新简介" {
				t.Fatal("审核失败仍写入")
			}
		})
	}
}

// TestIntegrationAllUGCEntrypoints 在正常权限和业务状态下验证文字入口全部拒绝且不会改变资金状态。
func TestIntegrationAllUGCEntrypoints(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	api.action("worker", o.ID, "accept", "", 200)
	api.action("worker", o.ID, "start", "", 200)
	completed := api.newOrder()
	api.action("worker", completed.ID, "accept", "", 200)
	api.action("worker", completed.ID, "start", "", 200)
	api.action("worker", completed.ID, "deliver", "已完成约定全部游戏任务", 200)
	api.action("player", completed.ID, "confirm", "", 200)
	installMockAI(t, api, func(*http.Request) (*http.Response, error) {
		return aiMockResponse(`{"decision":"reject","reason":"涉嫌违法信息，请修改内容"}`), nil
	})
	service := map[string]any{"game_id": "delta", "title": "需要审查的服务", "description": "提供正常组队陪练的游戏服务说明", "category": "任务协作", "price_cents": 2300, "unit": "局", "cover_url": "/static/covers/official/delta-2.jpg", "labels": []CoverLabel{}, "tags": []string{}, "club_id": "shanhai"}
	cases := []struct {
		user, method, path string
		body               any
	}{
		{"worker", "PATCH", "/me", map[string]string{"nickname": "游戏队友"}},
		{"worker", "PATCH", "/me/profile", map[string]string{"bio": "游戏任务服务介绍"}},
		{"worker", "POST", "/services", service}, {"worker", "PATCH", "/services/delta-service-1", service},
		{"worker", "POST", "/clubs", map[string]any{"name": "游戏俱乐部", "description": "这是俱乐部正常游戏服务说明", "game_ids": []string{"delta"}}},
		{"clubowner", "PATCH", "/clubs/shanhai", map[string]string{"assessment": "入会须完成正常游戏考核"}},
		{"outsider", "POST", "/clubs/shanhai/join", map[string]string{"application_note": "希望加入队伍进行游戏服务"}},
		{"clubowner", "POST", "/clubs/shanhai/members/worker", map[string]string{"status": "active", "review_note": "游戏考核已经通过"}},
		{"player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1, "requirements": "任务要求需要进行内容审核"}},
		{"player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "正常游戏需求", "requirements": "一起完成约定的游戏任务", "amount_cents": 1900}},
		{"worker", "POST", "/orders/" + o.ID + "/actions", map[string]string{"action": "deliver", "note": "已经完成游戏任务请验收"}},
		{"player", "POST", "/orders/" + o.ID + "/actions", map[string]string{"action": "request_refund", "note": "服务内容不符需要申请退款"}},
		{"player", "POST", "/orders/" + o.ID + "/messages", map[string]string{"content": "游戏聊天正文需要审查"}},
		{"player", "POST", "/orders/" + completed.ID + "/review", map[string]any{"rating": 5, "content": "服务评价内容需要审核"}},
	}
	for _, entry := range cases {
		t.Run(entry.method+entry.path, func(t *testing.T) {
			code, raw, err := api.request(entry.user, entry.method, entry.path, entry.body)
			if err != nil || code != 422 || !strings.Contains(string(raw), "moderation_rejected") || !strings.Contains(string(raw), "涉嫌违法信息") {
				t.Fatalf("入口绕过审查：%d %s %v", code, raw, err)
			}
		})
	}
	saved, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if saved.Status != "in_progress" || saved.RefundStatus != "none" {
		t.Fatal("拒绝后不应改变交易状态")
	}
}

// TestAIEncryptionPurposeBoundary 验证随机加密、篡改拒绝与支付配置密文不能互换。
func TestAIEncryptionPurposeBoundary(t *testing.T) {
	t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("e", 32))))
	value := defaultAISettings()
	value.APIKey = "private-test-ai-key"
	first, err := sealAISettings(value)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sealAISettings(value)
	if err != nil || first.Ciphertext == second.Ciphertext {
		t.Fatal("配置加密未使用随机nonce")
	}
	opened, err := openAISettings(first)
	if err != nil || opened.APIKey != value.APIKey {
		t.Fatal("配置无法正确解密")
	}
	if _, err = openPaymentSettings(first); err == nil {
		t.Fatal("AI配置密文被当作支付配置接受")
	}
	raw, _ := base64.StdEncoding.DecodeString(first.Ciphertext)
	raw[len(raw)-1] ^= 1
	first.Ciphertext = base64.StdEncoding.EncodeToString(raw)
	if _, err = openAISettings(first); err == nil {
		t.Fatal("篡改审核配置未被拒绝")
	}
	t.Setenv("CONFIG_ENCRYPTION_KEY", "invalid")
	if _, err = sealAISettings(value); err == nil {
		t.Fatal("无主密钥仍保存AI秘密")
	}
}
