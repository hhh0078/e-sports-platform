// Package app 使用一次性签名密钥和受控传输验证补款、部分退款、未知结果与原单补偿，不发生真实扣费。
package app

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// signedAdjustmentResponse 返回经过微信平台测试私钥签名的本地模拟响应。
func signedAdjustmentResponse(t *testing.T, c wxPayConfig, key *rsa.PrivateKey, value any) *http.Response {
	t.Helper()
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, raw), Body: io.NopCloser(bytes.NewReader(raw))}
}

// paidWXAdjustmentOrder 创建真实协议的原付款，回调仍通过生产验签与隔离测试库入账。
func paidWXAdjustmentOrder(t *testing.T, api *testAPI, c wxPayConfig, key *rsa.PrivateKey) Order {
	t.Helper()
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &o)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]string{"channel": "wechat"}, 200, nil)
	stored, e := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	v := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: o.OrderNo, TransactionID: "WXBASE" + o.ID, TradeState: "SUCCESS"}
	v.Payer.OpenID = stored.PayOpenID
	v.Amount.Total = o.AmountCents
	v.Amount.Currency = "CNY"
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", v, "payment"); code != 204 {
		t.Fatalf("原支付签名通知失败:%d", code)
	}
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	return o
}

// TestIntegrationOrderAdjustmentsWXUnknownRefund 验证独立补款验签、篡改防护、部分退款超时冻结、查询恢复和多原单全退。
func TestIntegrationOrderAdjustmentsWXUnknownRefund(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	mockAllowAI(t, api)
	var mu sync.Mutex
	refunds := map[string]wxRefund{}
	timeoutOnce := true
	posts := 0
	queries := 0
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" && r.URL.Path == "/v3/pay/transactions/jsapi" {
			return signedAdjustmentResponse(t, c, key, map[string]string{"prepay_id": "isolated-adjustment-prepay"}), nil
		}
		mu.Lock()
		defer mu.Unlock()
		if r.Method == "POST" && r.URL.Path == "/v3/refund/domestic/refunds" {
			var in struct {
				TransactionID string                        `json:"transaction_id"`
				RefundNo      string                        `json:"out_refund_no"`
				Amount        struct{ Total, Refund int64 } `json:"amount"`
			}
			if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
				t.Error(e)
			}
			var p OrderPayment
			if e := api.App.DB.QueryRow(context.Background(), "SELECT data FROM order_payments WHERE data->>'transaction_id'=$1", in.TransactionID).Scan(&p); e != nil {
				// pgx不能直接把JSONB扫描进业务结构时，使用通用持久化查询保持同一真实表证据。
				items, e := list[OrderPayment](context.Background(), api.App.DB, "order_payments", "WHERE data->>'transaction_id'=$1", in.TransactionID)
				if e != nil || len(items) != 1 {
					t.Errorf("退款找不到原单:%v", e)
				} else {
					p = items[0]
				}
			}
			result := wxRefund{MerchantID: c.MerchantID, OrderNo: p.OrderNo, OutRefundNo: in.RefundNo, TransactionID: in.TransactionID, Status: "SUCCESS"}
			result.Amount.Total = in.Amount.Total
			result.Amount.Refund = in.Amount.Refund
			result.Amount.Currency = "CNY"
			refunds[in.RefundNo] = result
			posts++
			if timeoutOnce {
				timeoutOnce = false
				return nil, context.DeadlineExceeded
			}
			return signedAdjustmentResponse(t, c, key, result), nil
		}
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v3/refund/domestic/refunds/") {
			queries++
			number := strings.TrimPrefix(r.URL.Path, "/v3/refund/domestic/refunds/")
			result, found := refunds[number]
			if !found {
				t.Error("查询绕过原退款号")
			}
			if queries == 1 {
				result.Status = "ABNORMAL"
			}
			return signedAdjustmentResponse(t, c, key, result), nil
		}
		t.Errorf("出现未预期出网请求:%s %s", r.Method, r.URL.Path)
		return nil, context.Canceled
	})}
	o := paidWXAdjustmentOrder(t, api, c, key)
	initial := o.AmountCents
	v := proposeTestAdjustment(api, o, "worker", "increase", "wx-adjustment-increase", 500)
	respondTestAdjustment(api, v, "player", "accept", 200)
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": "wechat"}, 200, nil)
	p, e := get[OrderPayment](t.Context(), api.App.DB, "order_payments", adjustmentPaymentID(v.ID), false)
	if e != nil {
		t.Fatal(e)
	}
	paid := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: p.OrderNo, TransactionID: "WXSUPPLEMENT" + p.ID, TradeState: "SUCCESS"}
	paid.Payer.OpenID = p.PayOpenID
	paid.Amount.Total = p.AmountCents
	paid.Amount.Currency = "CNY"
	bad := paid
	bad.Amount.Total++
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", bad, "payment"); code != 400 {
		t.Fatalf("补款错金额未拒绝:%d", code)
	}
	bad = paid
	bad.Payer.OpenID = "other-payer"
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", bad, "payment"); code != 400 {
		t.Fatalf("补款错付款人未拒绝:%d", code)
	}
	for i := 0; i < 2; i++ {
		if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", paid, "payment"); code != 204 {
			t.Fatalf("补款验签失败:%d", code)
		}
	}
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	if o.AmountCents != initial+500 {
		t.Fatal("补款没有核验入账或重复入账")
	}
	basePayment, e := get[OrderPayment](t.Context(), api.App.DB, "order_payments", walletKey("base-order-payment", o.ID), false)
	if e != nil {
		t.Fatal(e)
	}
	baseReplay := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: basePayment.OrderNo, TransactionID: basePayment.TransactionID, TradeState: "SUCCESS"}
	baseReplay.Payer.OpenID = basePayment.PayOpenID
	baseReplay.Amount.Total = basePayment.AmountCents
	baseReplay.Amount.Currency = "CNY"
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", baseReplay, "payment"); code != 204 {
		t.Fatal("净价变化后原支付重复通知没有幂等")
	}
	v = proposeTestAdjustment(api, o, "player", "decrease", "wx-adjustment-decrease", 200)
	respondTestAdjustment(api, v, "worker", "accept", 502)
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	if o.ActiveAdjustmentID != v.ID || o.AmountCents != initial+500 {
		t.Fatal("退款未知结果提前改价或解冻")
	}
	api.action("player", o.ID, "cancel", "", 409)
	api.action("worker", o.ID, "accept", "", 409)
	api.call("clubowner", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, 403, nil)
	var final OrderAdjustment
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, 200, &final)
	if final.Status != "refunding" || final.FailureReason == "" {
		t.Fatal("官方异常退款未保持冻结或缺少原因")
	}
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, 200, &final)
	if final.Status != "completed" {
		t.Fatal("退款原单查询未补偿")
	}
	mu.Lock()
	if posts != 1 || queries != 2 {
		t.Errorf("未知结果生成第二退款单或没有查询:%d %d", posts, queries)
	}
	var replay wxRefund
	for _, item := range refunds {
		replay = item
	}
	mu.Unlock()
	replay.RefundStatus = "SUCCESS"
	badRefund := replay
	badRefund.Amount.Refund++
	if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", badRefund, "refund"); code != 400 {
		t.Fatal("部分退款错金额未拒绝")
	}
	if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", replay, "refund"); code != 204 {
		t.Fatal("重复退款通知没有幂等")
	}
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	if o.AmountCents != initial+300 || o.PriceRefundedCents != 200 {
		t.Fatal("退款没有更新净价")
	}
	o = api.action("player", o.ID, "cancel", "取消后退还所有剩余原支付", 200)
	if o.Status != "refunded" {
		t.Fatal("多笔原单全额退款失败")
	}
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", baseReplay, "payment"); code != 204 {
		t.Fatal("退款后迟到原付款通知没有幂等")
	}
	payments, e := list[OrderPayment](t.Context(), api.App.DB, "order_payments", "WHERE data->>'order_id'=$1", o.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range payments {
		if item.RefundedCents != item.AmountCents || item.ReservedCents != 0 {
			t.Fatalf("全退漏掉原单:%+v", item)
		}
	}
	if testWallet(api, "player").RechargeCents != 0 || testWallet(api, "player").AvailableCents != 0 {
		t.Fatal("微信原路退款错误变成平台余额")
	}
}

// TestIntegrationOrderAdjustmentsWXExpiry 验证补款已出网时必须获得官方关单或原单不存在证据才能到期解冻。
func TestIntegrationOrderAdjustmentsWXExpiry(t *testing.T) {
	for _, scenario := range []string{"notpay", "signed_missing", "unsigned_missing"} {
		t.Run(scenario, func(t *testing.T) {
			api, c, key := productionWalletAPI(t)
			mockAllowAI(t, api)
			api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
				return signedAdjustmentResponse(t, c, key, map[string]string{"prepay_id": "expiry-order-prepay"}), nil
			})}
			o := paidWXAdjustmentOrder(t, api, c, key)
			v := proposeTestAdjustment(api, o, "worker", "increase", "wx-expiry-"+scenario, 100)
			respondTestAdjustment(api, v, "player", "accept", 200)
			api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": "wechat"}, 200, nil)
			v, e := get[OrderAdjustment](t.Context(), api.App.DB, "order_adjustments", v.ID, false)
			if e != nil {
				t.Fatal(e)
			}
			v.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
			if e = put(t.Context(), api.App.DB, "order_adjustments", v.ID, v); e != nil {
				t.Fatal(e)
			}
			p, e := get[OrderPayment](t.Context(), api.App.DB, "order_payments", adjustmentPaymentID(v.ID), false)
			if e != nil {
				t.Fatal(e)
			}
			closed := false
			api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
				if scenario != "notpay" {
					response := signedAdjustmentResponse(t, c, key, map[string]string{"code": "ORDER_NOT_EXIST"})
					response.StatusCode = 404
					if scenario == "unsigned_missing" {
						response.Header = http.Header{}
					}
					if r.Method == "POST" {
						closed = true
					}
					return response, nil
				}
				if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/close") {
					closed = true
					return signedAdjustmentResponse(t, c, key, map[string]string{}), nil
				}
				result := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: p.OrderNo, TradeState: "NOTPAY"}
				result.Amount.Total = p.AmountCents
				result.Amount.Currency = "CNY"
				return signedAdjustmentResponse(t, c, key, result), nil
			})}
			expected := 200
			if scenario == "unsigned_missing" {
				expected = 502
			}
			api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, expected, nil)
			api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
			if scenario == "unsigned_missing" {
				if o.ActiveAdjustmentID == "" || closed {
					t.Fatal("未签名404被当作官方关单")
				}
			} else {
				if o.ActiveAdjustmentID != "" || !closed {
					t.Fatal("官方关单未解除过期补款")
				}
			}
		})
	}
}
