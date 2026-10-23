// Package app 用可控交错传输复现补款建单与到期关闭的并发，验证迟到实付总能隔离入账并退款。
package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestIntegrationOrderAdjustmentsLateCreateAndRefund 验证创建在途不解锁、返回前复核、受控过期及迟到款不污染新协商。
func TestIntegrationOrderAdjustmentsLateCreateAndRefund(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	mockAllowAI(t, api)
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		return signedAdjustmentResponse(t, c, key, map[string]string{"prepay_id": "base-isolated-prepay"}), nil
	})}
	o := paidWXAdjustmentOrder(t, api, c, key)
	original := o.AmountCents
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-late-create", 250)
	respondTestAdjustment(api, v, "player", "accept", 200)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan int, 1)
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Path != "/v3/pay/transactions/jsapi" {
			t.Error("创建在途时不应同步关单")
			return nil, context.Canceled
		}
		var body struct {
			ExpiresAt string `json:"time_expire"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		deadline, e := time.Parse(time.RFC3339, body.ExpiresAt)
		if e != nil || time.Until(deadline) < 23*time.Hour {
			t.Error("没有传递固定原补款有效期")
		}
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return signedAdjustmentResponse(t, c, key, map[string]string{"prepay_id": "late-isolated-prepay"}), nil
	})}
	go func() {
		status, _, e := api.request("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": "wechat"})
		if e != nil {
			done <- 0
		} else {
			done <- status
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("补款创建未开始")
	}
	stored, e := get[OrderAdjustment](t.Context(), api.App.DB, "order_adjustments", v.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	if !adjustmentCreationActive(stored) {
		t.Fatal("出网之前没有持久化创建租约")
	}
	// 旧请求结束只能清除自己的租约，不能把较新的在途请求解冻。
	unchanged, finishErr := api.App.finishAdjustmentCreation(t.Context(), o.ID, v.ID, "older-request-lease")
	if finishErr != nil || unchanged.PaymentCreatingUntil != stored.PaymentCreatingUntil {
		t.Fatal("旧请求清除了新创建租约")
	}
	stored.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if e = put(t.Context(), api.App.DB, "order_adjustments", stored.ID, stored); e != nil {
		t.Fatal(e)
	}
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, 409, nil)
	current, e := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if e != nil || current.ActiveAdjustmentID != v.ID {
		t.Fatal("创建在途调价被提前清除")
	}
	close(release)
	if status := <-done; status != 409 {
		t.Fatalf("过期创建返回了可付款会话:%d", status)
	}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		response := signedAdjustmentResponse(t, c, key, map[string]string{"code": "ORDER_NOT_EXIST"})
		response.StatusCode = 404
		return response, nil
	})}
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, 200, nil)
	stored, e = get[OrderAdjustment](t.Context(), api.App.DB, "order_adjustments", v.ID, false)
	if e != nil || stored.Status != "expired" {
		t.Fatal("已无在途创建的官方不存在原单不能受控恢复")
	}
	// 旧付款结束后用户能提出新协商，随后旧付款即使出现异常迟到成功也不能覆盖新协商。
	next := proposeTestAdjustment(api, o, "player", "decrease", "adjust-after-late-expiry", 50)
	p, e := get[OrderPayment](t.Context(), api.App.DB, "order_payments", adjustmentPaymentID(v.ID), false)
	if e != nil {
		t.Fatal(e)
	}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })}
	paid := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: p.OrderNo, TransactionID: "WXLATE" + p.ID, TradeState: "SUCCESS"}
	paid.Amount.Total = p.AmountCents
	paid.Amount.Currency = "CNY"
	paid.Payer.OpenID = p.PayOpenID
	if status := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", paid, "payment"); status != 204 {
		t.Fatalf("已鉴真的迟到实付被丢弃:%d", status)
	}
	p, e = get[OrderPayment](t.Context(), api.App.DB, "order_payments", p.ID, false)
	if e != nil || p.Status != "paid" || p.Kind != "late_supplement" || p.ReservedCents != p.AmountCents {
		t.Fatal("迟到原款未隔离登记并预留退款")
	}
	current, e = get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if e != nil || current.AmountCents != original || current.PaidTotalCents != original || current.ActiveAdjustmentID != next.ID {
		t.Fatal("迟到款改变了净价、GMV或新协商")
	}
	lots, e := fundingLots(t.Context(), api.App.DB, "overpayment:"+p.ID, "overpayment")
	if e != nil || len(lots) != 1 || lots[0].RemainingCents != p.AmountCents {
		t.Fatal("迟到款没有独立来源批次")
	}
	if e = verifyFundingLot(t.Context(), api.App.DB, lots[0], false, map[string]bool{}); e == nil {
		t.Fatal("迟到隔离根被当作可提收入")
	}
	var analytics AnalyticsResult
	api.call("admin", "GET", "/admin/analytics", nil, 200, &analytics)
	if analytics.Summary.GMVCents != original || analytics.LegacyUntrackedOrders != 0 {
		t.Fatalf("隔离款污染原订单交易统计:%+v", analytics)
	}
	parts, e := list[OrderPartRefund](t.Context(), api.App.DB, "order_refunds", "WHERE data->>'group_id'=$1", "late:"+p.ID)
	if e != nil || len(parts) != 1 {
		t.Fatal("缺少迟到款原路退款记录")
	}
	refund := parts[0]
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, refund.RefundNo) {
			t.Error("迟到款没有查询原退款单")
		}
		result := wxRefund{MerchantID: c.MerchantID, OrderNo: p.OrderNo, OutRefundNo: refund.RefundNo, TransactionID: p.TransactionID, Status: "SUCCESS"}
		result.Amount.Total = p.AmountCents
		result.Amount.Refund = p.AmountCents
		result.Amount.Currency = "CNY"
		return signedAdjustmentResponse(t, c, key, result), nil
	})}
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/sync", map[string]any{}, 200, &stored)
	if stored.Status != "refunded" {
		t.Fatal("迟到款原路退款没有终态")
	}
	current, e = get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if e != nil || current.AmountCents != original || current.ActiveAdjustmentID != next.ID {
		t.Fatal("迟到退款覆盖新调价或原订单")
	}
	if testWallet(api, "worker").PendingCents != original {
		t.Fatal("迟到付款或退款污染服务收入")
	}
	if status := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", paid, "payment"); status != 204 {
		t.Fatal("重复迟到支付未幂等确认")
	}
}
