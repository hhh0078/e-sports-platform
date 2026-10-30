// Package app 通过隔离账本验证 Apple 退款问询仅按真实充值与服务履约证据给出建议，不请求外部资金接口。
package app

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// iosRefundAdviceFixture 建立已核验 Apple 充值；需要履约时通过正式资金分配方法建立服务消费来源。
func iosRefundAdviceFixture(t *testing.T, api *testAPI, fulfilled bool) (WalletTopup, virtualTransaction, Order) {
	t.Helper()
	p := WalletTopup{ID: "ios-advice-topup", AccountID: "player", UserID: "player", RequestID: "ios-advice-request", Purpose: "balance", Channel: "virtual", AmountCents: 10000, Status: "paid", OrderNo: "VCIOSADVICE", TransactionID: "WXIOSADVICE", OfferID: "123456", MerchantID: "virtual:123456", AppID: "wx_ios_advice_test", PayOpenID: "private-ios-advice-openid", CreatedAt: now(), UpdatedAt: now()}
	proof := virtualTransaction{ID: p.ID, TopupID: p.ID, Order: virtualCashOrder{OrderID: p.OrderNo, Status: 2, OrderFee: p.AmountCents, PaidFee: p.AmountCents, OrderType: 7, PaidTime: time.Now().Add(-24 * time.Hour).Unix(), WXOrderID: p.TransactionID, WXPayOrderID: "WXPayIOSADVICE", LeftFee: p.AmountCents}, CheckedAt: now()}
	o := Order{ID: "ios-advice-order", OrderNo: "IOSADVICESERVICE", BuyerID: p.UserID, ProviderID: "worker", AmountCents: p.AmountCents, Status: "completed", PaymentStatus: "paid", PaymentChannel: "virtual", CompletedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), RefundStatus: "none", CreatedAt: now(), UpdatedAt: now()}
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if err = lockWalletUsers(t.Context(), tx, p.UserID, o.ProviderID); err != nil {
		t.Fatal(err)
	}
	if err = put(t.Context(), tx, "wallet_topups", p.ID, p); err != nil {
		t.Fatal(err)
	}
	if err = put(t.Context(), tx, "virtual_transactions", proof.ID, proof); err != nil {
		t.Fatal(err)
	}
	if err = addFundingRoot(t.Context(), tx, "topup", p.ID, p.TransactionID, p.AmountCents, "virtual", p.UserID, "virtual_recharge", "", p.MerchantID, p.AppID); err != nil {
		t.Fatal(err)
	}
	wallet, err := lockedWallet(t.Context(), tx, p.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if err = appendWalletEntry(t.Context(), tx, &wallet, WalletEntry{Kind: "virtual_topup", ReferenceID: p.ID, AmountCents: p.AmountCents, VirtualDelta: p.AmountCents, Note: "隔离测试的已核验 Apple 现金充值"}); err != nil {
		t.Fatal(err)
	}
	if fulfilled {
		if err = put(t.Context(), tx, "orders", o.ID, o); err != nil {
			t.Fatal(err)
		}
		if err = accountVirtualCoins(t.Context(), tx, p.UserID, virtualConsumptionID(o.ID, ""), o.ID, p.AmountCents); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	return p, proof, o
}

// forbidIOSAdviceNetwork 确保退款问询只使用已有可信账本，不出网获取证据或触发真实资金动作。
func forbidIOSAdviceNetwork(t *testing.T, api *testAPI) {
	t.Helper()
	var calls atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("Apple问询隔离测试禁止外部请求")
	})}
	t.Cleanup(func() {
		if calls.Load() != 0 {
			t.Error("Apple退款建议不应调用外部接口")
		}
	})
}

// TestIntegrationVirtualIOSAdviceUnknownOrder 未知或空原单没有履约证据，保持建议退款。
func TestIntegrationVirtualIOSAdviceUnknownOrder(t *testing.T) {
	api := newTestAPI(t)
	forbidIOSAdviceNetwork(t, api)
	for _, id := range []string{"", "unknown-original-order", strings.Repeat("x", 129)} {
		advice := api.App.virtualIOSRefundAdvice(t.Context(), id)
		if advice.ResultCode != 0 || advice.ResultInfo == "" {
			t.Fatal("未知充值原单被无证据建议拒绝退款")
		}
	}
}

// TestIntegrationVirtualIOSAdviceUnspentCash 已付款但未消费的金币不能被当作已经完成服务。
func TestIntegrationVirtualIOSAdviceUnspentCash(t *testing.T) {
	api := newTestAPI(t)
	forbidIOSAdviceNetwork(t, api)
	p, proof, _ := iosRefundAdviceFixture(t, api, false)
	for _, id := range []string{p.OrderNo, p.TransactionID, proof.Order.WXPayOrderID} {
		if advice := api.App.virtualIOSRefundAdvice(t.Context(), id); advice.ResultCode != 0 {
			t.Fatal("未消费现金充值被当作已履约，或官方原单别名匹配错误")
		}
	}
}

// TestIntegrationVirtualIOSAdviceFulfilledService 只有该充值实际分配至已验收服务时给出结合履约范围审核的拒绝建议。
func TestIntegrationVirtualIOSAdviceFulfilledService(t *testing.T) {
	api := newTestAPI(t)
	forbidIOSAdviceNetwork(t, api)
	p, proof, _ := iosRefundAdviceFixture(t, api, true)
	for _, id := range []string{p.OrderNo, p.TransactionID, proof.Order.WXPayOrderID} {
		advice := api.App.virtualIOSRefundAdvice(t.Context(), id)
		if advice.ResultCode != 1 || !strings.Contains(advice.ResultInfo, "履约") || !strings.Contains(advice.Evidence, "Apple") {
			t.Fatal("已核验的实际履约未形成有边界的退款审核建议")
		}
		if strings.Contains(advice.Evidence, p.PayOpenID) {
			t.Fatal("退款问询证据暴露私密微信身份")
		}
	}
}

// TestIntegrationVirtualIOSAdviceRefundOrDispute 同一履约服务处于售后、已经退款或原现金来源冻结时不得继续建议拒绝。
func TestIntegrationVirtualIOSAdviceRefundOrDispute(t *testing.T) {
	api := newTestAPI(t)
	forbidIOSAdviceNetwork(t, api)
	p, proof, order := iosRefundAdviceFixture(t, api, true)
	for _, status := range []string{"requested", "escalated", "processing"} {
		current := order
		current.RefundStatus = status
		if err := put(t.Context(), api.App.DB, "orders", current.ID, current); err != nil {
			t.Fatal(err)
		}
		if advice := api.App.virtualIOSRefundAdvice(t.Context(), p.OrderNo); advice.ResultCode != 0 {
			t.Fatalf("进行中售后 %s 被忽略", status)
		}
	}
	refunded := order
	refunded.Status, refunded.PaymentStatus, refunded.RefundStatus = "refunded", "refunded", "approved"
	if err := put(t.Context(), api.App.DB, "orders", refunded.ID, refunded); err != nil {
		t.Fatal(err)
	}
	if advice := api.App.virtualIOSRefundAdvice(t.Context(), p.OrderNo); advice.ResultCode != 0 {
		t.Fatal("已退款服务仍被当作有效履约拒绝证据")
	}
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	proof.ReconciliationRequired = true
	if err := put(t.Context(), api.App.DB, "virtual_transactions", proof.ID, proof); err != nil {
		t.Fatal(err)
	}
	if advice := api.App.virtualIOSRefundAdvice(t.Context(), p.OrderNo); advice.ResultCode != 0 {
		t.Fatal("待核账现金来源仍可成为拒绝退款证据")
	}
}
