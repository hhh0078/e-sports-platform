// Package app 使用固定签名向量和腾讯云协议替身验证真实短信适配、限流、失败、过期及验证码防重放。
package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// configurePhoneSMSForTest 仅注入虚构凭据；发送请求必须由测试 HTTP 替身接管。
func configurePhoneSMSForTest(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{"TENCENT_SMS_SECRET_ID": "test-sms-id", "TENCENT_SMS_SECRET_KEY": "test-sms-secret", "TENCENT_SMS_SDK_APP_ID": "1400000000", "TENCENT_SMS_SIGN_NAME": "测试签名", "TENCENT_SMS_TEMPLATE_ID": "100", "TENCENT_SMS_REGION": "ap-guangzhou"} {
		t.Setenv(key, value)
	}
}

// TestPhoneSMSAuthorizationVector 核对独立 Python HMAC 实现所得固定向量，覆盖 UTC 日期和最终请求字节签名。
func TestPhoneSMSAuthorizationVector(t *testing.T) {
	body := []byte(`{"PhoneNumberSet":["+8613800138000"],"SmsSdkAppId":"1400000000","SignName":"test","TemplateId":"100","TemplateParamSet":["123456"]}`)
	got := smsAuthorization(phoneSMSConfig{SecretID: "test-sms-id", SecretKey: "test-sms-secret"}, time.Unix(1700000000, 0).In(time.FixedZone("Asia/Shanghai", 8*3600)), body)
	want := "TC3-HMAC-SHA256 Credential=test-sms-id/2023-11-14/sms/tc3_request, SignedHeaders=content-type;host;x-tc-action, Signature=21117eede5aa161ea668034debf7f98b2940f45472bb6f32a8604815efa60659"
	if got != want {
		t.Fatal("腾讯云TC3签名不符合固定向量")
	}
}

// TestIntegrationPhoneSMSVerificationLifecycle 验证不模拟短信、限流、号码归属、错误次数、真实认证和一次性消费。
func TestIntegrationPhoneSMSVerificationLifecycle(t *testing.T) {
	api := newTestAPI(t)
	t.Setenv("TENCENT_SMS_SECRET_ID", "")
	api.call("player", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13900139000"}, 503, nil)
	configurePhoneSMSForTest(t)
	var captured atomic.Value
	var calls atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Scheme != "https" || r.URL.Host != "sms.tencentcloudapi.com" || r.URL.Path != "/" || r.Method != "POST" || r.Header.Get("X-TC-Action") != "SendSms" || r.Header.Get("X-TC-Version") != "2021-01-11" || r.Header.Get("X-TC-Region") != "ap-guangzhou" || !strings.HasPrefix(r.Header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=test-sms-id/") {
			t.Error("短信调用偏离腾讯云固定协议")
		}
		var body struct {
			Phone      []string `json:"PhoneNumberSet"`
			Params     []string `json:"TemplateParamSet"`
			AppID      string   `json:"SmsSdkAppId"`
			SignName   string   `json:"SignName"`
			TemplateID string   `json:"TemplateId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Phone) != 1 || len(body.Params) != 1 || !phoneSMSCodePattern.MatchString(body.Params[0]) || body.AppID != "1400000000" || body.SignName != "测试签名" || body.TemplateID != "100" {
			t.Error("短信模板或号码字段无效")
		}
		captured.Store(body.Params[0])
		raw, _ := json.Marshal(map[string]any{"Response": map[string]any{"SendStatusSet": []any{map[string]string{"Code": "Ok", "PhoneNumber": body.Phone[0], "SerialNo": "test-sms-receipt"}}}})
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})}
	api.call("", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13900139000"}, 401, nil)
	api.call("player", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "invalid"}, 400, nil)
	var sent struct {
		Expires    int `json:"expires_in"`
		RetryAfter int `json:"retry_after"`
	}
	api.call("player", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13900139000"}, 200, &sent)
	if sent.Expires != 300 || sent.RetryAfter != 60 || calls.Load() != 1 {
		t.Fatal("验证码有效期或发送次数错误")
	}
	code := captured.Load().(string)
	stored := api.App.Redis.HGetAll(t.Context(), api.App.redisKey("phone-sms:code:player")).Val()
	if stored["digest"] == "" || stored["digest"] == code || stored["phone"] != "13900139000" {
		t.Fatal("Redis未保存有归属的验证码摘要")
	}
	api.call("player", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13900139000"}, 429, nil)
	api.call("worker", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13900139000"}, 429, nil)
	if calls.Load() != 1 {
		t.Fatal("限流后仍调用付费短信")
	}
	api.call("worker", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13900139000", "code": code}, 422, nil)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	api.call("player", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13900139000", "code": wrong}, 422, nil)
	var user User
	api.call("player", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13900139000", "code": code}, 200, &user)
	if !user.PhoneVerified || user.PhoneMasked != "139****9000" {
		t.Fatal("短信验证未生成真实脱敏认证")
	}
	api.call("player", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13900139000", "code": code}, 422, nil)
	identity, err := get[map[string]string](t.Context(), api.App.DB, "identities", "player", false)
	if err != nil || identity["phone_source"] != "sms_tencent" || identity["phone_number"] != "13900139000" {
		t.Fatal("短信认证来源未真实保存")
	}
	api.App.Config.DevMode = false
	if err = api.App.requireVerifiedPhone(httptest.NewRequest("POST", "/orders", nil), user); err != nil {
		t.Fatal("正式环境拒绝真实短信验证来源")
	}
	api.App.Config.DevMode = true
	api.call("worker", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13700137000"}, 200, nil)
	workerCode := captured.Load().(string)
	wrong = "000000"
	if wrong == workerCode {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		api.call("worker", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13700137000", "code": wrong}, 422, nil)
	}
	api.call("worker", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13700137000", "code": workerCode}, 422, nil)
	api.call("outsider", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13600136000"}, 200, nil)
	api.App.Redis.ExpireAt(t.Context(), api.App.redisKey("phone-sms:code:outsider"), time.Now().Add(-time.Second))
	api.call("outsider", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13600136000", "code": captured.Load().(string)}, 422, nil)
}

// TestIntegrationPhoneSMSFailureDoesNotAuthorize 验证上游拒绝与跳转不会落下可用验证码或伪造真实验证。
func TestIntegrationPhoneSMSFailureDoesNotAuthorize(t *testing.T) {
	api := newTestAPI(t)
	configurePhoneSMSForTest(t)
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"Response":{"Error":{"Code":"FailedOperation.InsufficientBalanceInSmsPackage","Message":"upstream-private-detail"}}}`))}, nil
	})}
	status, raw, err := api.request("player", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13900139000"})
	if err != nil || status != 502 || strings.Contains(string(raw), "upstream-private-detail") {
		t.Fatal("失败短信响应错误或泄露上游细节")
	}
	if api.App.Redis.Exists(t.Context(), api.App.redisKey("phone-sms:code:player")).Val() != 0 {
		t.Fatal("上游失败后创建了可用验证码")
	}
	api.call("player", "POST", "/me/phone/sms/verify", map[string]string{"phone_number": "13900139000", "code": "123456"}, 422, nil)
	var redirected atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "sms.tencentcloudapi.com" {
			redirected.Add(1)
		}
		return &http.Response{StatusCode: 307, Header: http.Header{"Location": []string{"https://evil.example/sms"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	api.call("worker", "POST", "/me/phone/sms/send", map[string]string{"phone_number": "13700137000"}, 502, nil)
	if redirected.Load() != 0 {
		t.Fatal("短信凭据被转发给重定向目标")
	}
}
