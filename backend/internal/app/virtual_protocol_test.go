// Package app 验证虚拟支付签名、整金币金额边界、现金终态与网络失败时的安全关闭。
package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestVirtualSignatureVector 使用公开HMAC固定向量核对签名，改动原始字节必须改变签名。
func TestVirtualSignatureVector(t *testing.T) {
	if got := virtualHMAC("key", "The quick brown fox jumps over the lazy dog"); got != "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8" {
		t.Fatal("HMAC签名不符合独立固定向量")
	}
	if virtualHMAC("key", `requestVirtualPayment&{"env":0}`) == virtualHMAC("key", `requestVirtualPayment&{ "env":0}`) {
		t.Fatal("签名忽略了原始JSON字节")
	}
}

// TestVirtualQuoteMoneyBoundaries 验证默认平台承担、两端费率、可用金币抵扣及非整金币拒绝。
func TestVirtualQuoteMoneyBoundaries(t *testing.T) {
	c := commerceSettings{Mode: "mini_program", FeeBearer: "platform", AndroidFeeBPS: 100, IOSFeeBPS: 1200}
	q, err := makeVirtualQuote(c, 10000, 3000, "ios")
	if err != nil || q.FeeCents != 0 || q.TotalCents != 10000 || q.TopupCents != 7000 || q.FeeBPS != 1200 {
		t.Fatal("平台承担费用时改变了买家服务总价")
	}
	c.FeeBearer = "buyer"
	q, err = makeVirtualQuote(c, 10000, 12000, "ios")
	if err != nil || q.FeeCents != 1200 || q.TotalCents != 11200 || q.TopupCents != 0 {
		t.Fatal("买家承担的总金币或充值差额错误")
	}
	q, err = makeVirtualQuote(c, 10000, 0, "android")
	if err != nil || q.FeeCents != 100 || q.TotalCents != 10100 {
		t.Fatal("安卓与iOS费率没有独立应用")
	}
	for _, input := range []struct {
		service, balance int64
		platform         string
	}{{0, 0, "android"}, {-100, 0, "ios"}, {10000001, 0, "ios"}, {100, -1, "android"}, {100, 0, "unknown"}, {990, 0, "android"}, {6800, 0, "android"}} {
		if _, err = makeVirtualQuote(c, input.service, input.balance, input.platform); err == nil {
			t.Fatal("无效金额、终端或非整金币费用被接受")
		}
	}
}

// TestVirtualCashOrderEvidence 拒绝扣币单、未付款、退款终态、篡改金额及未来支付时间。
func TestVirtualCashOrderEvidence(t *testing.T) {
	p := WalletTopup{Channel: "virtual", Purpose: "balance", OrderNo: "VC12345678", AmountCents: 10000}
	good := virtualCashOrder{OrderID: p.OrderNo, Status: 2, OrderFee: 10000, PaidFee: 10000, OrderType: 7, WXOrderID: "verified-platform-order", PaidTime: time.Now().Unix()}
	if err := validateVirtualCashOrder(p, good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*virtualCashOrder){
		func(v *virtualCashOrder) { v.Status = 1 }, func(v *virtualCashOrder) { v.Status = 5 }, func(v *virtualCashOrder) { v.Status = 8 },
		func(v *virtualCashOrder) { v.OrderFee = 9999 }, func(v *virtualCashOrder) { v.PaidFee = 9999 }, func(v *virtualCashOrder) { v.OrderType = 8 },
		func(v *virtualCashOrder) { v.OrderID = "another-order" }, func(v *virtualCashOrder) { v.WXOrderID = "" }, func(v *virtualCashOrder) { v.PaidTime = time.Now().Add(time.Hour).Unix() },
	} {
		v := good
		change(&v)
		if validateVirtualCashOrder(p, v) == nil {
			t.Fatal("无法核对的现金充值被当作金币入账证据")
		}
	}
}

// TestIntegrationVirtualProtocolUsesExactBytes 用隔离Redis及固定上游替身核验签名与禁止重定向，不发真实支付。
func TestIntegrationVirtualProtocolUsesExactBytes(t *testing.T) {
	api := newTestAPI(t)
	api.App.Config.WechatAppID = "wx_virtual_protocol"
	api.App.Config.WechatSecret = "only-test-secret"
	appKey, sessionKey := "virtual-test-app-key-1234567890", "private-test-session"
	api.App.commerceConfig = &commerceSettings{Mode: "mini_program", FeeBearer: "platform", OfferID: "12345", AppKey: appKey}
	responses := []string{`{"errcode":0,"balance":100}`, `{"errcode":268490006}`, `{"unexpected":0}`}
	requests := 0
	redirect := false
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "api.weixin.qq.com" {
			t.Fatal("支付秘密被发送到非官方地址")
		}
		if r.URL.Path == "/cgi-bin/stable_token" {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"access_token":"test-access","expires_in":7200}`)), Request: r}, nil
		}
		requests++
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if r.URL.Path != "/xpay/query_user_balance" || r.URL.Query().Get("access_token") != "test-access" || r.URL.Query().Get("pay_sig") != virtualHMAC(appKey, r.URL.Path+"&"+string(raw)) || r.URL.Query().Get("signature") != virtualHMAC(sessionKey, string(raw)) {
			t.Fatal("实际请求字节或URI未被完整签名")
		}
		if redirect {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://another.invalid/secret"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		body := responses[requests-1]
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	var out struct {
		Balance int `json:"balance"`
	}
	body := map[string]any{"openid": "private-user", "env": 0, "remark": "中文 & 空格"}
	code, err := api.App.virtualRequest(t.Context(), api.App.Config.WechatAppID, "12345", 0, "/xpay/query_user_balance", body, sessionKey, &out)
	if err != nil || code != 0 || out.Balance != 100 {
		t.Fatal("有效官方响应未被解析")
	}
	code, err = api.App.virtualRequest(t.Context(), api.App.Config.WechatAppID, "12345", 0, "/xpay/query_user_balance", body, sessionKey, nil)
	if err != nil || code != 268490006 || virtualResultError(code) == nil {
		t.Fatal("上游业务失败被当作成功")
	}
	if _, err = api.App.virtualRequest(t.Context(), api.App.Config.WechatAppID, "12345", 0, "/xpay/query_user_balance", body, sessionKey, nil); err == nil {
		t.Fatal("缺少errcode的响应被接受")
	}
	redirect = true
	if _, err = api.App.virtualRequest(t.Context(), api.App.Config.WechatAppID, "12345", 0, "/xpay/query_user_balance", body, sessionKey, nil); err == nil {
		t.Fatal("支付请求接受了重定向")
	}
	if requests != 4 {
		t.Fatal("重定向导致额外请求")
	}
	if _, err = api.App.virtualRequest(t.Context(), api.App.Config.WechatAppID, "12345", 0, "/xpay/unknown", body, sessionKey, nil); err == nil || requests != 4 {
		t.Fatal("未知上游路径触发了请求")
	}
}
