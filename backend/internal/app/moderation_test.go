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

// TestIntegrationAIConfigPrivacyAndManualFallback 验证缺AI仍保存、管理员配置加密与真实连通测试不使用模拟。
func TestIntegrationAIConfigPrivacyAndManualFallback(t *testing.T) {
	api := newTestAPI(t)
	for _, user := range []string{"", "player", "worker", "clubowner", "outsider"} {
		want := 403
		if user == "" {
			want = 401
		}
		api.call(user, "PATCH", "/admin/ai-settings", map[string]string{"api_key": "must-not-save"}, want, nil)
		api.call(user, "GET", "/admin/ai-settings", nil, want, nil)
	}
	var view struct {
		Model, Mode string
		HasAPIKey   bool `json:"has_api_key"`
	}
	api.call("admin", "GET", "/admin/ai-settings", nil, 200, &view)
	if view.Model != defaultAIModel || view.Mode != "manual" || view.HasAPIKey {
		t.Fatal("默认应人工审核")
	}
	api.call("admin", "POST", "/admin/ai-settings/test", map[string]any{}, 503, nil)
	api.App.Config.DevMode = false
	code, raw, err := api.request("worker", "PATCH", "/me/profile", map[string]string{"bio": "正式环境缺AI配置仍正常保存"})
	if err != nil || code != 200 || !strings.Contains(string(raw), "manual_pending") {
		t.Fatalf("缺AI阻断保存：%d %s %v", code, raw, err)
	}
	saved, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
	if saved.Bio != "正式环境缺AI配置仍正常保存" {
		t.Fatal("资料未写入")
	}
	mockAllowAI(t, api)
	api.call("admin", "POST", "/admin/ai-settings/test", map[string]any{}, 200, nil)
	for _, path := range []string{"/admin/ai-settings", "/admin/settings", "/bootstrap", "/admin/audit", "/admin/moderation"} {
		code, raw, err = api.request("admin", "GET", path, nil)
		if err != nil || code != 200 || strings.Contains(string(raw), "ai-test-secret-not-public") {
			t.Fatalf("配置秘密泄漏%s", path)
		}
	}
	encrypted, err := get[encryptedPaymentSettings](t.Context(), api.App.DB, "settings", "ai-private", false)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if strings.Contains(string(decoded), "ai-test-secret-not-public") {
		t.Fatal("密钥未加密")
	}
	api.call("admin", "PATCH", "/admin/ai-settings", map[string]string{"api_key": "", "model": "new-model"}, 200, nil)
	cfg, err := api.App.readAISettings(t.Context(), api.App.DB)
	if err != nil || cfg.APIKey != "ai-test-secret-not-public" {
		t.Fatal("空密钥覆盖旧值")
	}
	api.call("admin", "PATCH", "/admin/ai-settings", map[string]bool{"clear_api_key": true}, 200, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "清除密钥后仍保存进入人工队列"}, 200, nil)
}

// drainReviewAI 在隔离测试库显式消费持久队列，既不启动后台线程也不绕过AI协议解析。
func drainReviewAI(t *testing.T, api *testAPI) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		processed, err := api.App.processModerationOne(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !processed {
			return
		}
	}
	t.Fatal("审核队列未收敛")
}

// TestIntegrationAIHTTPProtocolAndManualFallback 验证实际图文协议与上游故障进入人工而不回滚保存。
func TestIntegrationAIHTTPProtocolAndManualFallback(t *testing.T) {
	api := newTestAPI(t)
	var calls atomic.Int32
	var last string
	installMockAI(t, api, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		last = string(raw)
		if r.Method != "POST" || r.URL.String() != defaultAIBaseURL+"/chat/completions" || r.Header.Get("Authorization") != "Bearer ai-test-secret-not-public" {
			t.Error("AI协议错误")
		}
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "正常游戏任务服务介绍"}, 200, nil)
	if calls.Load() != 0 {
		t.Fatal("保存接口仍同步调用AI")
	}
	drainReviewAI(t, api)
	if !strings.Contains(last, "正常游戏任务服务介绍") || strings.Contains(last, "openid") || strings.Contains(last, "phone_number") || strings.Contains(last, "ai-test-secret-not-public") {
		t.Fatal("AI载荷缺失或带无关秘密")
	}
	o := api.newOrder()
	api.uploadTestImage("player", o.ID)
	drainReviewAI(t, api)
	if !strings.Contains(last, "data:image/png;base64,") {
		t.Fatal("没有审核实际图片字节")
	}
	before := calls.Load()
	api.call("outsider", "POST", "/orders/"+o.ID+"/messages", map[string]string{"content": "不能越权提交"}, 403, nil)
	drainReviewAI(t, api)
	if calls.Load() != before {
		t.Fatal("越权内容进入模型")
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
			api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "上游故障也保存的新简介" + scenario}, 200, nil)
			drainReviewAI(t, api)
			task := latestReviewTask(t, api, "users", "worker", "bio")
			if task.Status != "manual_pending" || strings.Contains(task.LastError, "ai-test-secret-not-public") {
				t.Fatal("故障未转人工或泄密")
			}
			saved, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
			if saved.Bio != "上游故障也保存的新简介"+scenario {
				t.Fatal("审核故障阻断保存")
			}
		})
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
