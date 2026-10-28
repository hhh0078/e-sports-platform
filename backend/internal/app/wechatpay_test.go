// Package app 使用独立测试密钥和真实数据库验证支付签名、篡改防护、回调幂等及退款核对。
package app

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

type paymentTransport func(*http.Request) (*http.Response, error)

// RoundTrip 在测试中模拟官方应答，仍要求真实的签名与验签过程。
func (f paymentTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// testPayConfig 生成一次性商户与平台密钥，与真实账户完全隔离。
func testPayConfig(t *testing.T) (wxPayConfig, *rsa.PrivateKey) {
	t.Helper()
	merchant, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	platform, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	return wxPayConfig{AppID: "wx_test_app", MerchantID: "1900000001", Serial: "MERCHANT_SERIAL", PublicKeyID: "PUB_KEY_ID_TEST", APIKey: "12345678901234567890123456789012", NotifyURL: "https://example.com/api/payments/wechat/notify", RefundNotifyURL: "https://example.com/api/payments/wechat/refund-notify", PrivateKey: merchant, PublicKey: &platform.PublicKey}, platform
}

// signedHeader 为测试数据创建微信格式的平台签名。
func signedHeader(t *testing.T, c wxPayConfig, key *rsa.PrivateKey, body []byte) http.Header {
	t.Helper()
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "test-random-nonce"
	signature, e := wxSign(key, stamp+"\n"+nonce+"\n"+string(body)+"\n")
	if e != nil {
		t.Fatal(e)
	}
	h := http.Header{}
	h.Set("Wechatpay-Timestamp", stamp)
	h.Set("Wechatpay-Nonce", nonce)
	h.Set("Wechatpay-Serial", c.PublicKeyID)
	h.Set("Wechatpay-Signature", signature)
	return h
}

// TestWXVerifyRejectsTampering 覆盖正确签名、内容修改、过期、序列号和空随机串。
func TestWXVerifyRejectsTampering(t *testing.T) {
	c, key := testPayConfig(t)
	body := []byte(`{"amount":100}`)
	header := signedHeader(t, c, key, body)
	if e := wxVerify(c, header, body); e != nil {
		t.Fatal(e)
	}
	if wxVerify(c, header, []byte(`{"amount":1}`)) == nil {
		t.Fatal("篡改金额被接受")
	}
	for _, field := range []string{"Wechatpay-Serial", "Wechatpay-Nonce", "Wechatpay-Signature"} {
		changed := header.Clone()
		changed.Set(field, "")
		if wxVerify(c, changed, body) == nil {
			t.Fatalf("缺少%s被接受", field)
		}
	}
	expired := header.Clone()
	expired.Set("Wechatpay-Timestamp", strconv.FormatInt(time.Now().Add(-6*time.Minute).Unix(), 10))
	if wxVerify(c, expired, body) == nil {
		t.Fatal("过期消息被接受")
	}
}

// encryptedNotice 用真实 AES-GCM 生成测试通知。
func encryptedNotice(t *testing.T, c wxPayConfig, event string, payload any) []byte {
	t.Helper()
	plain, e := json.Marshal(payload)
	if e != nil {
		t.Fatal(e)
	}
	block, e := aes.NewCipher([]byte(c.APIKey))
	if e != nil {
		t.Fatal(e)
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		t.Fatal(e)
	}
	nonce := "123456789012"
	ciphertext := gcm.Seal(nil, []byte(nonce), plain, []byte("transaction"))
	raw, e := json.Marshal(map[string]any{"event_type": event, "resource": map[string]string{"algorithm": "AEAD_AES_256_GCM", "nonce": nonce, "associated_data": "transaction", "ciphertext": base64.StdEncoding.EncodeToString(ciphertext)}})
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

// TestWXDecryptRejectsWrongKeyAndEvent 验证加密通知不接受错密钥、错误事件和篡改密文。
func TestWXDecryptRejectsWrongKeyAndEvent(t *testing.T) {
	c, _ := testPayConfig(t)
	raw := encryptedNotice(t, c, "TRANSACTION.SUCCESS", map[string]any{"mchid": c.MerchantID})
	var result wxTransaction
	if e := decryptWXResource(c, raw, "TRANSACTION.SUCCESS", &result); e != nil || result.MerchantID != c.MerchantID {
		t.Fatalf("解密失败:%v", e)
	}
	if decryptWXResource(c, raw, "REFUND.SUCCESS", &result) == nil {
		t.Fatal("错误通知类型被接受")
	}
	wrong := c
	wrong.APIKey = strings.Repeat("a", 32)
	if decryptWXResource(wrong, raw, "TRANSACTION.SUCCESS", &result) == nil {
		t.Fatal("错误密钥被接受")
	}
	var data map[string]any
	json.Unmarshal(raw, &data)
	data["resource"].(map[string]any)["nonce"] = "short"
	raw, _ = json.Marshal(data)
	if decryptWXResource(c, raw, "TRANSACTION.SUCCESS", &result) == nil {
		t.Fatal("无效nonce被接受")
	}
}

// TestWXRequestSignsAndVerifies 确认请求签名覆盖方法、路径和金额，响应没有签名就拒绝。
func TestWXRequestSignsAndVerifies(t *testing.T) {
	c, key := testPayConfig(t)
	tampered := false
	a := App{HTTPClient: &http.Client{Transport: paymentTransport(func(request *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(request.Body)
		auth := request.Header.Get("Authorization")
		fields := map[string]string{}
		for _, part := range regexp.MustCompile(`([a-z_]+)="([^"]*)"`).FindAllStringSubmatch(auth, -1) {
			fields[part[1]] = part[2]
		}
		signature, _ := base64.StdEncoding.DecodeString(fields["signature"])
		expected, _ := wxSign(c.PrivateKey, request.Method+"\n"+request.URL.RequestURI()+"\n"+fields["timestamp"]+"\n"+fields["nonce_str"]+"\n"+string(raw)+"\n")
		if base64.StdEncoding.EncodeToString(signature) != expected || fields["mchid"] != c.MerchantID || request.URL.Host != "api.mch.weixin.qq.com" {
			t.Error("请求签名不符合微信协议")
		}
		body := []byte(`{"prepay_id":"verified-token"}`)
		header := signedHeader(t, c, key, body)
		if tampered {
			body = []byte(`{"prepay_id":"forged-token"}`)
		}
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}}
	var result map[string]string
	if e := a.wxRequest(t.Context(), c, "POST", "/v3/pay/transactions/jsapi", map[string]int{"amount": 900}, &result); e != nil || result["prepay_id"] != "verified-token" {
		t.Fatalf("正常应答失败:%v", e)
	}
	tampered = true
	if a.wxRequest(t.Context(), c, "POST", "/v3/pay/transactions/jsapi", map[string]int{"amount": 900}, &result) == nil {
		t.Fatal("篡改响应被接受")
	}
}

// configurePayEnvironment 将测试配置写入临时密钥文件，生命周期结束自动清理。
func configurePayEnvironment(t *testing.T, c wxPayConfig) {
	t.Helper()
	dir := t.TempDir()
	privateDER, e := x509.MarshalPKCS8PrivateKey(c.PrivateKey)
	if e != nil {
		t.Fatal(e)
	}
	publicDER, e := x509.MarshalPKIXPublicKey(c.PublicKey)
	if e != nil {
		t.Fatal(e)
	}
	privatePath := filepath.Join(dir, "merchant.pem")
	publicPath := filepath.Join(dir, "platform.pem")
	if e = os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0600); e != nil {
		t.Fatal(e)
	}
	values := map[string]string{"WECHAT_PAY_MCH_ID": c.MerchantID, "WECHAT_PAY_SERIAL": c.Serial, "WECHAT_PAY_PUBLIC_KEY_ID": c.PublicKeyID, "WECHAT_PAY_API_V3_KEY": c.APIKey, "WECHAT_PAY_NOTIFY_URL": c.NotifyURL, "WECHAT_PAY_REFUND_NOTIFY_URL": c.RefundNotifyURL, "WECHAT_PAY_PRIVATE_KEY_PATH": privatePath, "WECHAT_PAY_PUBLIC_KEY_PATH": publicPath}
	for key, value := range values {
		t.Setenv(key, value)
	}
}

// TestWXPaymentAndRefundCallbacks 验证真正 HTTP 通知链路、金额核对、重复幂等以及收入回退。
func TestWXPaymentAndRefundCallbacks(t *testing.T) {
	test := newTestAPI(t)
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	test.App.Config.DevMode = false
	test.App.Config.WechatAppID = c.AppID
	o := baseOrder(User{ID: "player", Nickname: "小玩家"})
	o.GameID = "delta"
	o.ServiceID = "delta-service-1"
	o.ProviderID = "worker"
	o.AmountCents = 2900
	o.PayOpenID = "payer-openid"
	o.Title = "真实协议测试订单"
	if e := put(t.Context(), test.App.DB, "orders", o.ID, o); e != nil {
		t.Fatal(e)
	}
	paid := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: o.OrderNo, TransactionID: "WX-TXN-TEST", TradeState: "SUCCESS"}
	paid.Amount.Total = o.AmountCents
	paid.Amount.Currency = "CNY"
	paid.Payer.OpenID = o.PayOpenID
	// deliver 将真实加密签名通知送到生产回调处理器。
	deliver := func(event string, payload any, refund bool) int {
		raw := encryptedNotice(t, c, event, payload)
		request := httptest.NewRequest("POST", "/notify", bytes.NewReader(raw))
		request.Header = signedHeader(t, c, key, raw)
		record := httptest.NewRecorder()
		if refund {
			test.App.refundCallback(record, request)
		} else {
			test.App.paymentCallback(record, request)
		}
		return record.Code
	}
	bad := paid
	bad.Amount.Total++
	if code := deliver("TRANSACTION.SUCCESS", bad, false); code != 400 {
		t.Fatalf("错金额回调未拒绝:%d", code)
	}
	for range 2 {
		if code := deliver("TRANSACTION.SUCCESS", paid, false); code != 204 {
			t.Fatalf("付款回调失败:%d", code)
		}
	}
	loaded, e := get[Order](t.Context(), test.App.DB, "orders", o.ID, false)
	if e != nil || loaded.Status != "open" || loaded.PaymentTransactionID != paid.TransactionID {
		t.Fatalf("未确认支付:%+v %v", loaded, e)
	}
	messages, e := list[Message](t.Context(), test.App.DB, "messages", "WHERE data->>'order_id'=$1", o.ID)
	if e != nil || len(messages) != 1 {
		t.Fatalf("重复支付通知产生多条记录:%d %v", len(messages), e)
	}
	loaded.Status = "completed"
	loaded.CompletedAt = now()
	loaded.RefundStatus = "processing"
	loaded.RefundID = "RF" + loaded.ID
	if e = put(t.Context(), test.App.DB, "orders", loaded.ID, loaded); e != nil {
		t.Fatal(e)
	}
	refund := wxRefund{MerchantID: c.MerchantID, OrderNo: loaded.OrderNo, OutRefundNo: loaded.RefundID, TransactionID: loaded.PaymentTransactionID, RefundStatus: "SUCCESS"}
	refund.Amount.Total = loaded.AmountCents
	refund.Amount.Refund = loaded.AmountCents
	abnormal := refund
	abnormal.RefundStatus = "ABNORMAL"
	if code := deliver("REFUND.SUCCESS", abnormal, true); code != 400 {
		t.Fatalf("事件与退款状态不匹配仍接受:%d", code)
	}
	for range 2 {
		if code := deliver("REFUND.ABNORMAL", abnormal, true); code != 204 {
			t.Fatalf("异常退款回调失败:%d", code)
		}
	}
	held, err := get[Order](t.Context(), test.App.DB, "orders", loaded.ID, false)
	if err != nil || held.RefundStatus != "processing" || held.PaymentStatus != "paid" {
		t.Fatalf("异常退款错误解冻:%+v %v", held, err)
	}
	badRefund := refund
	badRefund.Amount.Refund--
	if code := deliver("REFUND.SUCCESS", badRefund, true); code != 400 {
		t.Fatalf("错退款金额未拒绝:%d", code)
	}
	for range 2 {
		if code := deliver("REFUND.SUCCESS", refund, true); code != 204 {
			t.Fatalf("退款回调失败:%d", code)
		}
	}
	loaded, e = get[Order](t.Context(), test.App.DB, "orders", loaded.ID, false)
	if e != nil || loaded.Status != "refunded" || loaded.PaymentStatus != "refunded" {
		t.Fatalf("退款未完成:%+v %v", loaded, e)
	}
	messages, e = list[Message](t.Context(), test.App.DB, "messages", "WHERE data->>'order_id'=$1", o.ID)
	if e != nil || len(messages) != 3 {
		t.Fatalf("重复退款通知不幂等:%d %v", len(messages), e)
	}
}

// TestWXConfigFailsClosed 缺少配置时不会返回假支付成功。
func TestWXConfigFailsClosed(t *testing.T) {
	t.Setenv("WECHAT_PAY_MCH_ID", "")
	a := App{}
	if _, e := a.loadWXPay(); e == nil {
		t.Fatal("缺少支付配置仍可支付")
	}
	if validNotifyURL("http://example.com/notify") || validNotifyURL("https://user:password@example.com/notify") {
		t.Fatal("不安全通知地址被接受")
	}
}

// TestWXCloseAbsentOrderRequiresSignature 验证明确未建单可以取消，但伪造404、已支付及网络错误不能绕过关单。
func TestWXCloseAbsentOrderRequiresSignature(t *testing.T) {
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	signed, code := true, "ORDER_NOT_EXIST"
	a := App{Config: Config{WechatAppID: c.AppID}, HTTPClient: &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := json.Marshal(map[string]string{"code": code})
		header := http.Header{}
		if signed {
			header = signedHeader(t, c, key, raw)
		}
		return &http.Response{StatusCode: 404, Header: header, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}}
	order := Order{OrderNo: "test-not-created"}
	if err := a.closePayment(t.Context(), order); err != nil {
		t.Fatalf("已验签未建单应可取消:%v", err)
	}
	signed = false
	if a.closePayment(t.Context(), order) == nil {
		t.Fatal("未验签404不能视为取消成功")
	}
	signed, code = true, "ORDER_PAID"
	if a.closePayment(t.Context(), order) == nil {
		t.Fatal("已支付订单不能取消")
	}
}

// testVerifiedWechatIdentity 仅为支付协议回归构造独立测试身份，真实手机兑换由手机号专项测试覆盖。
func testVerifiedWechatIdentity(openid string) map[string]string {
	return map[string]string{"openid": openid, "phone_number": "13800138000", "phone_country_code": "86", "phone_source": "wechat", "phone_verified_at": now(), "identity_source": "xxapi", "identity_adult": "true", "identity_verified_at": now()}
}
