// Package app 验证虚拟现金退款的累计查询与单笔通知交错时不会重复扣币，账本与原现金退款总额始终守恒。
package app

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
)

// assertVirtualCashRefundState 同时核对钱包、原充值、核验凭据、退款分录和剩余来源，避免只检查一个展示余额。
func assertVirtualCashRefundState(t *testing.T, api *testAPI, p WalletTopup, refunded, notified int64) {
	t.Helper()
	wallet, err := get[Wallet](t.Context(), api.App.DB, "wallets", p.UserID, false)
	if err != nil {
		t.Fatal(err)
	}
	current, err := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := get[virtualTransaction](t.Context(), api.App.DB, "virtual_transactions", p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.VirtualCoinCents != p.AmountCents-refunded || current.RefundedCents != refunded || proof.CashRefundCents != refunded || proof.RecoveredCoinCents != refunded || proof.NotifiedRefundCents != notified {
		t.Fatalf("退款重复扣币或累计额错误：余额=%d，原单退款=%d，现金退款=%d，撤回金币=%d，通知累计=%d；期望余额=%d，退款=%d，通知=%d", wallet.VirtualCoinCents, current.RefundedCents, proof.CashRefundCents, proof.RecoveredCoinCents, proof.NotifiedRefundCents, p.AmountCents-refunded, refunded, notified)
	}
	if wallet.VirtualCoinCents+proof.CashRefundCents != p.AmountCents || !proof.ReconciliationRequired {
		t.Fatal("现金退款与剩余金币不守恒，或退款后来源未冻结核账")
	}
	var returnedEntries, remainingLots int64
	if err = api.App.DB.QueryRow(t.Context(), "SELECT COALESCE(sum((data->>'virtual_delta')::bigint),0) FROM wallet_entries WHERE data->>'user_id'=$1 AND data->>'kind'='virtual_cash_refund'", p.UserID).Scan(&returnedEntries); err != nil {
		t.Fatal(err)
	}
	if err = api.App.DB.QueryRow(t.Context(), "SELECT COALESCE(sum((data->>'remaining_cents')::bigint),0) FROM fund_lots WHERE data->>'receipt_id'=$1 AND data->>'account_id'=$2 AND data->>'bucket'='virtual_recharge'", walletKey("receipt", "topup", p.ID), p.UserID).Scan(&remainingLots); err != nil {
		t.Fatal(err)
	}
	if returnedEntries != -refunded || remainingLots != wallet.VirtualCoinCents {
		t.Fatalf("退款账本来源不守恒：退款金币分录合计=%d，剩余来源=%d，钱包=%d", returnedEntries, remainingLots, wallet.VirtualCoinCents)
	}
}

// TestIntegrationVirtualCashRefundQueryNotificationInterleave 覆盖先查后通知、先通知后查及重复通知，原现金退款只撤回一次对应金币。
func TestIntegrationVirtualCashRefundQueryNotificationInterleave(t *testing.T) {
	api := newTestAPI(t)
	forbidIOSAdviceNetwork(t, api)
	p, _, _ := iosRefundAdviceFixture(t, api, false)
	steps := []struct {
		name       string
		amount     int64
		event      string
		cumulative bool
		refunded   int64
		notified   int64
	}{
		{"累计查询1000", 1000, "query:refund-total:1000", true, 1000, 0},
		{"同笔通知A1000", 1000, "WxRefundIdA", false, 1000, 1000},
		{"累计查询2000", 2000, "query:refund-total:2000", true, 2000, 1000},
		{"同笔通知B1000", 1000, "WxRefundIdB", false, 2000, 2000},
		{"新退款通知C1000", 1000, "WxRefundIdC", false, 3000, 3000},
		{"累计查询3000", 3000, "query:refund-total:3000", true, 3000, 3000},
		{"迟到旧查询1000", 1000, "query:stale-refund-total:1000", true, 3000, 3000},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			if err := api.App.recordVirtualCashRefund(t.Context(), p, step.amount, step.event, step.cumulative); err != nil {
				t.Fatal(err)
			}
			assertVirtualCashRefundState(t, api, p, step.refunded, step.notified)
		})
	}
	var entriesBefore, movementsBefore int64
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM wallet_entries WHERE data->>'user_id'=$1 AND data->>'kind'='virtual_cash_refund'", p.UserID).Scan(&entriesBefore); err != nil {
		t.Fatal(err)
	}
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM fund_movements").Scan(&movementsBefore); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"WxRefundIdA", "WxRefundIdB", "WxRefundIdC", "WxRefundIdA", "WxRefundIdB", "WxRefundIdC"} {
		if err := api.App.recordVirtualCashRefund(t.Context(), p, 1000, id, false); err != nil {
			t.Fatal(err)
		}
		assertVirtualCashRefundState(t, api, p, 3000, 3000)
	}
	var entriesAfter, movementsAfter int64
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM wallet_entries WHERE data->>'user_id'=$1 AND data->>'kind'='virtual_cash_refund'", p.UserID).Scan(&entriesAfter); err != nil {
		t.Fatal(err)
	}
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM fund_movements").Scan(&movementsAfter); err != nil {
		t.Fatal(err)
	}
	if entriesAfter != entriesBefore || movementsAfter != movementsBefore {
		t.Fatal("重复退款通知生成了新的扣币分录或资金转移")
	}
}

// TestIntegrationVirtualCashRefundSnapshotPreservesNotified 上游短暂返回旧已付快照时仍保留通知累计，下一笔通知不能被当作已经处理。
func TestIntegrationVirtualCashRefundSnapshotPreservesNotified(t *testing.T) {
	api := newTestAPI(t)
	p, originalProof, _ := iosRefundAdviceFixture(t, api, false)
	api.App.paymentConfigMu.Lock()
	api.App.commerceConfig = &commerceSettings{Mode: "mini_program", FeeBearer: "platform", OfferID: p.OfferID, AppKey: "cash-refund-local-test-key", VirtualEnv: p.VirtualEnv}
	api.App.paymentConfigMu.Unlock()
	var queries atomic.Int32
	mockWechat(t, api, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "cash-refund-local-token", "expires_in": 7200})
		case "/xpay/query_order":
			queries.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "order": originalProof.Order})
		default:
			t.Errorf("退款快照测试不应请求其他接口 %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	api.App.Config.WechatAppID = p.AppID
	if err := api.App.recordVirtualCashRefund(t.Context(), p, 1000, "snapshot-notify-A", false); err != nil {
		t.Fatal(err)
	}
	assertVirtualCashRefundState(t, api, p, 1000, 1000)
	if err := api.App.syncVirtualTopup(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if queries.Load() != 1 {
		t.Fatal("没有实际通过本地协议替身刷新已付原单快照")
	}
	assertVirtualCashRefundState(t, api, p, 1000, 1000)
	if err := api.App.recordVirtualCashRefund(t.Context(), p, 1000, "snapshot-notify-B", false); err != nil {
		t.Fatal(err)
	}
	assertVirtualCashRefundState(t, api, p, 2000, 2000)
}
