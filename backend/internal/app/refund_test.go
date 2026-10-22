// Package app 对真实退款通道注入受理后网络超时，验证先持久化意图和查询补偿。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestIntegrationRefundTimeoutRecovery 覆盖买家取消、卖家同意与平台裁决三个真实退款入口。
func TestIntegrationRefundTimeoutRecovery(t *testing.T) {
	for _, scenario := range []string{"cancel", "approve", "resolve"} {
		t.Run(scenario, func(t *testing.T) {
			api := newTestAPI(t)
			c, key := testPayConfig(t)
			configurePayEnvironment(t, c)
			o := api.newOrder()
			stored, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			stored.PaymentTransactionID = "wx-timeout-" + o.ID
			stored.PayOpenID = "buyer-openid"
			if scenario != "cancel" {
				stored.Status = "delivered"
				stored.RefundStatus = "requested"
			}
			if scenario == "resolve" {
				stored.RefundStatus = "escalated"
			}
			if err = put(t.Context(), api.App.DB, "orders", o.ID, stored); err != nil {
				t.Fatal(err)
			}
			mockAllowAI(t, api)
			api.App.Config.DevMode = false
			api.App.Config.WechatAppID = c.AppID
			var submitted atomic.Int32
			api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(req *http.Request) (*http.Response, error) {
				persisted, e := get[Order](context.Background(), api.App.DB, "orders", o.ID, false)
				if e != nil || persisted.RefundStatus != "processing" || persisted.RefundID != "RF"+o.ID || persisted.PaymentStatus != "paid" {
					t.Errorf("外部退款请求前未持久化冻结意图: %+v %v", persisted, e)
				}
				if req.Method == "POST" {
					submitted.Add(1)
					status, _, e := api.request("player", "POST", "/orders/"+o.ID+"/actions", map[string]string{"action": "confirm"})
					if e != nil || status != 409 {
						t.Errorf("退款受理时仍能结算: %d %v", status, e)
					}
					return nil, context.DeadlineExceeded
				}
				if req.Method != "GET" || !strings.Contains(req.URL.Path, "RF"+o.ID) {
					t.Errorf("资金同步没有查询同一退款号: %s", req.URL.Path)
				}
				refund := wxRefund{MerchantID: c.MerchantID, OrderNo: stored.OrderNo, OutRefundNo: "RF" + o.ID, TransactionID: stored.PaymentTransactionID, Status: "SUCCESS"}
				refund.Amount.Total = stored.AmountCents
				refund.Amount.Refund = stored.AmountCents
				refund.Amount.Currency = "CNY"
				raw, _ := json.Marshal(refund)
				return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, raw), Body: io.NopCloser(bytes.NewReader(raw))}, nil
			})}
			var result Order
			switch scenario {
			case "cancel":
				result = api.action("player", o.ID, "cancel", "", 200)
			case "approve":
				result = api.action("worker", o.ID, "approve_refund", "同意退还全部费用", 200)
			case "resolve":
				api.call("admin", "POST", "/admin/orders/"+o.ID+"/resolve", map[string]string{"resolution": "refund", "note": "平台核实证据同意退款"}, 200, &result)
			}
			if result.RefundStatus != "processing" || result.PaymentStatus != "paid" {
				t.Fatalf("未知外部结果应保持冻结: %+v", result)
			}
			api.action("player", o.ID, "withdraw_refund", "", 409)
			api.action("player", o.ID, "confirm", "", 409)
			api.call("player", "POST", "/orders/"+o.ID+"/sync-payment", map[string]any{}, 200, &result)
			if result.Status != "refunded" || result.PaymentStatus != "refunded" || submitted.Load() != 1 {
				t.Fatalf("查询补偿失败或重复外部提交: %+v submitted=%d", result, submitted.Load())
			}
		})
	}
}
