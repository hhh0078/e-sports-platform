// Package app 验证演示记录的所有资金与交易入口都拒绝写入，且不会向微信发起请求。
package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestRequireLiveOrderRejectsEveryDemoReference 验证演示标记在订单或任何关联资源上都不能进入交易。
func TestRequireLiveOrderRejectsEveryDemoReference(t *testing.T) {
	for _, order := range []Order{{ID: "demo_order"}, {ID: "real", BuyerID: "demo_buyer"}, {ID: "real", ProviderID: "demo_provider"}, {ID: "real", ServiceID: "demo_service"}, {ID: "real", ClubID: "demo_club"}} {
		if requireLiveOrder(order) == nil {
			t.Fatal("演示资源未被隔离")
		}
	}
	if err := requireLiveOrder(Order{ID: "real-order", BuyerID: "real-buyer"}); err != nil {
		t.Fatal(err)
	}
}

// TestIntegrationDemoTradeBlocked 覆盖付款、关单、接单、退款同步、裁决和回调，失败后数据库保持不变。
func TestIntegrationDemoTradeBlocked(t *testing.T) {
	api := newTestAPI(t)
	c, _ := testPayConfig(t)
	configurePayEnvironment(t, c)
	api.App.Config.WechatAppID = c.AppID
	api.App.Config.DevMode = false
	var remote atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		remote.Add(1)
		t.Error("演示订单触发外部资金请求")
		return nil, http.ErrNotSupported
	})}
	order := baseOrder(User{ID: "player", Nickname: "测试买家"})
	order.ID = "demo_trade_guard"
	order.ProviderID = "worker"
	order.Title = "演示订单"
	order.PayOpenID = "fake-openid"
	order.AmountCents = 100
	scenarios := []struct {
		user, path              string
		input                   any
		status, payment, refund string
	}{
		{"player", "/orders/" + order.ID + "/pay", map[string]any{}, "pending_payment", "unpaid", ""},
		{"player", "/orders/" + order.ID + "/actions", map[string]string{"action": "cancel"}, "pending_payment", "unpaid", ""},
		{"worker", "/orders/" + order.ID + "/actions", map[string]string{"action": "accept"}, "open", "paid", ""},
		{"outsider", "/orders/" + order.ID + "/actions", map[string]string{"action": "claim"}, "open", "paid", ""},
		{"worker", "/orders/" + order.ID + "/actions", map[string]string{"action": "approve_refund"}, "delivered", "paid", "requested"},
		{"admin", "/orders/" + order.ID + "/sync-payment", map[string]any{}, "delivered", "paid", "processing"},
		{"admin", "/admin/orders/" + order.ID + "/resolve", map[string]string{"resolution": "refund", "note": "演示不能退款"}, "delivered", "paid", "escalated"},
		{"admin", "/admin/orders/" + order.ID + "/retry-refund", map[string]any{}, "delivered", "paid", "processing"},
		{"admin", "/orders/" + order.ID + "/messages", map[string]string{"content": "不能修改演示沟通"}, "delivered", "paid", ""},
		{"admin", "/orders/" + order.ID + "/evidence", map[string]any{}, "delivered", "paid", ""},
		{"player", "/orders/" + order.ID + "/review", map[string]any{"rating": 5, "content": "不能评价演示订单"}, "completed", "paid", ""},
	}
	for _, scenario := range scenarios {
		order.Status = scenario.status
		order.PaymentStatus = scenario.payment
		order.RefundStatus = scenario.refund
		order.RefundID = "RF-demo-test"
		order.PaymentTransactionID = "demo-paid-txn"
		if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(order)
		api.call(scenario.user, "POST", scenario.path, scenario.input, 400, nil)
		saved, err := get[Order](t.Context(), api.App.DB, "orders", order.ID, false)
		after, _ := json.Marshal(saved)
		if err != nil || string(before) != string(after) {
			t.Fatalf("演示订单被写入: %s", scenario.path)
		}
	}
	order.Status = "pending_payment"
	order.PaymentStatus = "unpaid"
	order.PaymentTransactionID = ""
	put(t.Context(), api.App.DB, "orders", order.ID, order)
	payment := wxTransaction{MerchantID: c.MerchantID, AppID: c.AppID, TradeState: "SUCCESS", TransactionID: "demo-wx-txn", OrderNo: order.OrderNo}
	payment.Amount.Total = order.AmountCents
	payment.Amount.Currency = "CNY"
	payment.Payer.OpenID = order.PayOpenID
	if err := api.App.applyWXPayment(t.Context(), c, payment); err == nil || !strings.Contains(err.Error(), "演示") {
		t.Fatal("支付回调未拒绝演示订单")
	}
	order.Status = "delivered"
	order.PaymentStatus = "paid"
	order.PaymentTransactionID = "demo-wx-txn"
	order.RefundStatus = "processing"
	put(t.Context(), api.App.DB, "orders", order.ID, order)
	refund := wxRefund{MerchantID: c.MerchantID, OrderNo: order.OrderNo, OutRefundNo: order.RefundID, TransactionID: order.PaymentTransactionID, Status: "SUCCESS"}
	refund.Amount.Total = order.AmountCents
	refund.Amount.Refund = order.AmountCents
	refund.Amount.Currency = "CNY"
	if err := api.App.applyWXRefund(t.Context(), c, refund); err == nil || !strings.Contains(err.Error(), "演示") {
		t.Fatal("退款回调未拒绝演示订单")
	}
	if _, err := api.App.refund(t.Context(), order, "演示"); err == nil {
		t.Fatal("资金适配器接受演示退款")
	}
	if err := api.App.closePayment(t.Context(), order); err == nil {
		t.Fatal("资金适配器接受演示关单")
	}
	if remote.Load() != 0 {
		t.Fatal("演示记录产生外部请求")
	}
	for _, path := range []string{"/orders/" + order.ID, "/orders/" + order.ID + "/messages", "/orders/" + order.ID + "/reviews"} {
		api.call("admin", "GET", path, nil, 200, nil)
	}
	for _, table := range []string{"messages", "reviews", "evidence"} {
		var count int
		if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM "+validTable(table)+" WHERE data->>'order_id'=$1", order.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("演示订单新增了%s", table)
		}
	}
}

// TestIntegrationDemoResourceReferenceBlocked 验证普通编号订单也不能通过演示服务写入聊天、证据或统计。
func TestIntegrationDemoResourceReferenceBlocked(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.SeedShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	order := Order{ID: "real-id-demo-resource", OrderNo: "RESOURCE-GUARD-001", BuyerID: "player", ProviderID: "worker", ServiceID: "demo_service_001", Status: "completed", PaymentStatus: "paid", RefundStatus: "none"}
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		path string
		body any
	}{
		{"messages", map[string]string{"content": "不能修改演示沟通"}},
		{"evidence", map[string]any{}},
		{"review", map[string]any{"rating": 5, "content": "不能评价关联演示服务的订单"}},
	} {
		api.call("player", "POST", "/orders/"+order.ID+"/"+scenario.path, scenario.body, 409, nil)
	}
	for _, table := range []string{"messages", "reviews", "evidence"} {
		var count int
		if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM "+validTable(table)+" WHERE data->>'order_id'=$1", order.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("关联演示服务的订单新增了%s", table)
		}
	}
	counts, err := api.App.ShowcaseStatus(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for table, expected := range expectedShowcaseCounts() {
		if counts[table] != expected {
			t.Fatalf("%s 演示记录标记被改写", table)
		}
	}
}
