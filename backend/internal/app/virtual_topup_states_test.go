// Package app 验证金币原单在官方处理中、凭据传播、退款及迟到成功时均不会误报金额或重复入账。
package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestIntegrationVirtualTopupPropagationAndRefund 用受控查单覆盖状态传播和先退款后同步，绝不发起真实扣款。
func TestIntegrationVirtualTopupPropagationAndRefund(t *testing.T) {
	api, _, _, _ := virtualFundsAPI(t, true)
	var official virtualCashOrder
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "topup-state-test-token", "expires_in": 7200})
		case "/xpay/query_order":
			json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "order": official})
		default:
			t.Errorf("原单核验不应发起其它支付接口 %s", r.URL.Path)
			w.WriteHeader(400)
		}
	})
	for i, status := range []int{0, 1, 3, 6, 2, 4, 5, 8} {
		p := WalletTopup{ID: walletKey("state-test", string(rune('a'+i))), UserID: "player", AccountID: "player", RequestID: "state-" + string(rune('a'+i)), Channel: "virtual", Purpose: "balance", AmountCents: 100, OrderNo: "VCSTATE" + string(rune('A'+i)), OfferID: "12345", AppID: api.App.Config.WechatAppID, PayOpenID: "virtual-open-player", Status: "pending_payment", CreatedAt: now(), UpdatedAt: now()}
		if err := put(t.Context(), api.App.DB, "wallet_topups", p.ID, p); err != nil {
			t.Fatal(err)
		}
		official = virtualCashOrder{OrderID: p.OrderNo, Status: status, OrderFee: 100, PaidFee: 100, OrderType: 0, LeftFee: 100}
		if status == 5 || status == 8 {
			official.LeftFee = 0
			official.WXOrderID = "REFUNDTEST"
			official.PaidTime = time.Now().Unix()
		}
		for range 2 {
			if err := api.App.syncVirtualTopup(t.Context(), p); err != nil {
				t.Fatalf("状态%d不应误报金额不一致: %v", status, err)
			}
		}
		current, err := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", p.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		if testWallet(api, "player").VirtualCoinCents != 0 {
			t.Fatal("不完整或退款凭据产生了金币")
		}
		if status == 5 || status == 8 {
			if current.Status != "cash_refunded" || current.RefundedCents != 100 {
				t.Fatal("未入账原单退款没有正确记录")
			}
			official.Status = 4
			official.LeftFee = 100
			if err = api.App.syncVirtualTopup(t.Context(), p); err == nil {
				t.Fatal("迟到成功覆盖了退款终态")
			}
			continue
		}
		if current.Status != "pending_payment" {
			t.Fatal("处理中或凭据不完整改写了原意图")
		}
	}
	p := WalletTopup{Channel: "virtual", Purpose: "balance", OrderNo: "VCAMOUNT", AmountCents: 100}
	official = virtualCashOrder{OrderID: p.OrderNo, Status: 4, OrderFee: 101, PaidFee: 100}
	if err := validateVirtualCashOrder(p, official); err == nil || !strings.Contains(err.Error(), "面额或实付金额") {
		t.Fatal("真实金额冲突没有独立错误")
	}
}

// TestIntegrationVirtualTopupCompleteEvidenceIdempotent 验证买家手续费设置不会叠加购币面额，完整现金凭据只入账一次。
func TestIntegrationVirtualTopupCompleteEvidenceIdempotent(t *testing.T) {
	api, _, _, _ := virtualFundsAPI(t, true)
	p := WalletTopup{ID: walletKey("complete-state"), UserID: "player", AccountID: "player", Channel: "virtual", Purpose: "balance", AmountCents: 100, OrderNo: "VCCOMPLETE", OfferID: "12345", AppID: api.App.Config.WechatAppID, PayOpenID: "virtual-open-player", Status: "pending_payment", CreatedAt: now(), UpdatedAt: now()}
	if err := put(t.Context(), api.App.DB, "wallet_topups", p.ID, p); err != nil {
		t.Fatal(err)
	}
	official := virtualCashOrder{OrderID: p.OrderNo, Status: 4, OrderFee: 100, PaidFee: 100, OrderType: 0, LeftFee: 100, WXOrderID: "WXCASHCOMPLETE", PaidTime: time.Now().Unix()}
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "complete-state-test-token", "expires_in": 7200})
			return
		}
		if r.URL.Path != "/xpay/query_order" {
			t.Errorf("禁止额外扣款 %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "order": official})
	})
	api.App.Config.WechatAppID = p.AppID
	for range 3 {
		if err := api.App.syncVirtualTopup(t.Context(), p); err != nil {
			t.Fatal(err)
		}
	}
	if testWallet(api, "player").VirtualCoinCents != 100 {
		t.Fatal("服务手续费混入购币或重复入账")
	}
	current, err := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", p.ID, false)
	if err != nil || current.Status != "paid" || current.TransactionID != official.WXOrderID {
		t.Fatal("支付事实丢失")
	}
}
