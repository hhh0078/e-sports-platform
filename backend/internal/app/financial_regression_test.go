// Package app 回归验证服务组织并发锁序、历史实付退款兼容与公开需求已付平台费率快照。
package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// waitFinancialAdvisoryLock 等待请求确实阻塞在指定组织锁上，确保并发测试覆盖读后组织变更窗口。
func waitFinancialAdvisoryLock(t *testing.T, api *testAPI, club string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		e := api.App.DB.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND classid=((hashtextextended($1,0)>>32)&4294967295)::oid AND objid=(hashtextextended($1,0)&4294967295)::oid)`, "jijie-club-finance:"+club).Scan(&waiting)
		if e != nil {
			t.Fatal(e)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("并发请求未进入预期组织锁等待")
}

// TestIntegrationFinancialServiceClubChangeLockOrder 验证服务换组织后返回409而不在已持用户锁时申请新组织锁。
func TestIntegrationFinancialServiceClubChangeLockOrder(t *testing.T) {
	api := newTestAPI(t)
	oldLock, e := api.App.DB.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer oldLock.Rollback(t.Context())
	if e = lockClubFinance(t.Context(), oldLock, "shanhai"); e != nil {
		t.Fatal(e)
	}
	type result struct {
		code int
		body []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		code, body, e := api.request("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1})
		done <- result{code, body, e}
	}()
	waitFinancialAdvisoryLock(t, api, "shanhai")
	service, e := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	if e != nil {
		t.Fatal(e)
	}
	service.ClubID = "wild"
	if e = put(t.Context(), api.App.DB, "services", service.ID, service); e != nil {
		t.Fatal(e)
	}
	newLock, e := api.App.DB.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer newLock.Rollback(t.Context())
	if e = lockClubFinance(t.Context(), newLock, "wild"); e != nil {
		t.Fatal(e)
	}
	if e = oldLock.Commit(t.Context()); e != nil {
		t.Fatal(e)
	}
	select {
	case r := <-done:
		if r.err != nil || r.code != 409 || !strings.Contains(string(r.body), "俱乐部归属已更新") {
			t.Fatalf("服务变更未要求重新取锁:%d %s %v", r.code, r.body, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("服务换组织后仍等待新组织锁，存在club→user反向死锁风险")
	}
}

// TestIntegrationFinancialPublicRequestFrozenPlatformRates 验证已付需求领取只选择创建时两档费率，合法零费率及旧单版本均不套新价。
func TestIntegrationFinancialPublicRequestFrozenPlatformRates(t *testing.T) {
	api := newTestAPI(t)
	original := FinancialSettings{FreePlatformRateBPS: 500, ClubPlatformRateBPS: 800}
	api.call("admin", "PATCH", "/admin/financial-settings", original, 200, nil)
	club, _ := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	club.CommissionRateBPS = 1000
	put(t.Context(), api.App.DB, "clubs", club.ID, club)
	requests := []Order{}
	for i := 0; i < 2; i++ {
		var o Order
		api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": fmt.Sprintf("费率冻结测试需求%d", i), "requirements": "测试领取时的平台两档费率沿用创建快照", "amount_cents": 10000}, 200, &o)
		api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 200, nil)
		requests = append(requests, o)
	}
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{FreePlatformRateBPS: 2000, ClubPlatformRateBPS: 3000}, 200, nil)
	club.CommissionRateBPS = 1200
	put(t.Context(), api.App.DB, "clubs", club.ID, club)
	var affiliated, free Order
	api.call("worker", "POST", "/orders/"+requests[0].ID+"/actions", map[string]any{"action": "claim", "club_id": "shanhai"}, 200, &affiliated)
	api.call("outsider", "POST", "/orders/"+requests[1].ID+"/actions", map[string]any{"action": "claim"}, 200, &free)
	if affiliated.Settlement.PlatformRateBPS != 800 || affiliated.Settlement.ClubRateBPS != 1200 || affiliated.Settlement.ProviderIncomeCents != 8000 || free.Settlement.PlatformRateBPS != 500 {
		t.Fatalf("领取覆盖旧平台费率:%+v %+v", affiliated.Settlement, free.Settlement)
	}
	for _, version := range []int{1, 2} {
		api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{}, 200, nil)
		var o Order
		api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "合法零费率冻结测试", "requirements": "平台之后提费也不影响已付款需求的零费率", "amount_cents": 1000}, 200, &o)
		api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 200, nil)
		if version == 1 {
			stored, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
			stored.Settlement = OrderSettlement{Version: 1, PlatformRateBPS: 0, ProviderIncomeCents: 1000}
			put(t.Context(), api.App.DB, "orders", o.ID, stored)
		}
		api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{FreePlatformRateBPS: 900, ClubPlatformRateBPS: 1900}, 200, nil)
		var claimed Order
		api.call("worker", "POST", "/orders/"+o.ID+"/actions", map[string]any{"action": "claim", "club_id": "shanhai"}, 200, &claimed)
		if claimed.Settlement.PlatformRateBPS != 0 || claimed.Settlement.Version != 2 {
			t.Fatalf("版本%d零费率被覆盖:%+v", version, claimed.Settlement)
		}
	}
}

// legacyRefundFixture 构造旧版已实付托管与原待结算账单；旧JSON缺版本字段由升级迁移明确标记。
func legacyRefundFixture(t *testing.T, api *testAPI, c wxPayConfig, legacy bool) Order {
	t.Helper()
	ctx := t.Context()
	tx, e := api.App.DB.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = lockWalletUsers(ctx, tx, "player", "worker"); e != nil {
		t.Fatal(e)
	}
	id := newID()
	o := Order{ID: id, OrderNo: "LEGACY" + id, BuyerID: "player", ProviderID: "worker", AmountCents: 3800, Status: "refund_requested", PaymentStatus: "paid", RefundStatus: "processing", RefundID: "RF" + id, PaymentTransactionID: "WX" + id, PaymentMerchantID: c.MerchantID, PaymentAppID: c.AppID, PaymentChannel: "wechat", WalletAccounted: true, CreatedAt: now()}
	if e = put(ctx, tx, "orders", id, o); e != nil {
		t.Fatal(e)
	}
	wallet, e := lockedWallet(ctx, tx, "worker")
	if e != nil {
		t.Fatal(e)
	}
	if e = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "order_pending", ReferenceID: id, AmountCents: 3800, PendingDelta: 3800, Note: "升级前实付托管收入"}); e != nil {
		t.Fatal(e)
	}
	escrow := WalletEscrow{FundingVersion: 1, OrderID: id, BuyerID: "player", ProviderID: "worker", AmountCents: 3800, Channel: "wechat", Status: "holding", CreatedAt: now()}
	if legacy {
		raw, _ := json.Marshal(escrow)
		data := map[string]any{}
		json.Unmarshal(raw, &data)
		delete(data, "funding_version")
		if e = put(ctx, tx, "wallet_escrows", id, data); e != nil {
			t.Fatal(e)
		}
	} else {
		if e = put(ctx, tx, "wallet_escrows", id, escrow); e != nil {
			t.Fatal(e)
		}
		if e = addFundingRoot(ctx, tx, "order", id, o.PaymentTransactionID, o.AmountCents, "wechat", "escrow:"+id, "escrow", id, c.MerchantID, c.AppID); e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, "DELETE FROM fund_lots WHERE data->>'account_id'=$1", "escrow:"+id); e != nil {
			t.Fatal(e)
		}
	}
	if e = markLegacyFundingEscrows(ctx, tx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return o
}

// TestIntegrationFinancialLegacyVerifiedWXRefund 验证真实验签退款通知可关闭旧托管并撤回原pending，新账缺来源仍拒绝且不补造来源。
func TestIntegrationFinancialLegacyVerifiedWXRefund(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	old := legacyRefundFixture(t, api, c, true)
	refund := wxRefund{MerchantID: c.MerchantID, OrderNo: old.OrderNo, OutRefundNo: old.RefundID, TransactionID: old.PaymentTransactionID, RefundStatus: "SUCCESS"}
	refund.Amount.Total = 3800
	refund.Amount.Refund = 3800
	refund.Amount.Currency = "CNY"
	for i := 0; i < 2; i++ {
		if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", refund, "refund"); code != 204 {
			t.Fatalf("旧单官方退款未成功记账:%d", code)
		}
	}
	stored, _ := get[Order](t.Context(), api.App.DB, "orders", old.ID, false)
	escrow, _ := get[WalletEscrow](t.Context(), api.App.DB, "wallet_escrows", old.ID, false)
	wallet := testWallet(api, "worker")
	if stored.PaymentStatus != "refunded" || !stored.FinancialReconciliationRequired || escrow.Status != "refunded" || !escrow.ReconciliationRequired || wallet.PendingCents != 0 || wallet.AvailableCents != 0 {
		t.Fatalf("历史退款未隔离完成:%+v %+v %+v", stored, escrow, wallet)
	}
	var sources int
	if e := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM fund_lots WHERE data->>'order_id'=$1", old.ID).Scan(&sources); e != nil || sources != 0 {
		t.Fatal("旧退款补造了资金来源")
	}
	var audits int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM audit WHERE data->>'action'='wallet.legacy_refund' AND data->>'target_id'=$1", old.ID).Scan(&audits)
	if audits != 1 {
		t.Fatalf("历史退款审计不唯一:%d", audits)
	}
	damaged := legacyRefundFixture(t, api, c, false)
	refund.OrderNo = damaged.OrderNo
	refund.OutRefundNo = damaged.RefundID
	refund.TransactionID = damaged.PaymentTransactionID
	if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", refund, "refund"); code != 409 {
		t.Fatalf("新版损坏来源意外走历史兼容:%d", code)
	}
	check, _ := get[Order](t.Context(), api.App.DB, "orders", damaged.ID, false)
	if check.PaymentStatus != "paid" || testWallet(api, "worker").PendingCents != 3800 {
		t.Fatal("新版来源损坏时绕过了资金校验")
	}
}

// legacyBalanceRefundFixture 还原升级前混合余额实付的不可变付款和待结算分录，不创建新版来源批次。
func legacyBalanceRefundFixture(t *testing.T, api *testAPI) Order {
	t.Helper()
	ctx := t.Context()
	tx, e := api.App.DB.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = lockWalletUsers(ctx, tx, "player", "worker"); e != nil {
		t.Fatal(e)
	}
	id := newID()
	buyer, e := lockedWallet(ctx, tx, "player")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range []WalletEntry{{Kind: "legacy_fixture_recharge", ReferenceID: id, AmountCents: 2000, RechargeDelta: 2000, Note: "升级前充值存量夹具"}, {Kind: "legacy_fixture_income", ReferenceID: id, AmountCents: 1800, AvailableDelta: 1800, Note: "升级前收入存量夹具"}, {Kind: "order_payment", ReferenceID: id, AmountCents: -3800, RechargeDelta: -2000, AvailableDelta: -1800, Note: "升级前混合余额实付"}} {
		if e = appendWalletEntry(ctx, tx, &buyer, entry); e != nil {
			t.Fatal(e)
		}
	}
	provider, e := lockedWallet(ctx, tx, "worker")
	if e != nil {
		t.Fatal(e)
	}
	if e = appendWalletEntry(ctx, tx, &provider, WalletEntry{Kind: "order_pending", ReferenceID: id, AmountCents: 3800, PendingDelta: 3800, Note: "升级前余额订单待结算"}); e != nil {
		t.Fatal(e)
	}
	o := Order{ID: id, OrderNo: "LEGACYBAL" + id, BuyerID: "player", ProviderID: "worker", ServiceID: "delta-service-1", AmountCents: 3800, Status: "open", PaymentStatus: "paid", RefundStatus: "none", PaymentChannel: "balance", BalanceRechargeCents: 2000, BalanceIncomeCents: 1800, WalletAccounted: true, CreatedAt: now()}
	if e = put(ctx, tx, "orders", id, o); e != nil {
		t.Fatal(e)
	}
	escrow := WalletEscrow{OrderID: id, BuyerID: "player", ProviderID: "worker", AmountCents: 3800, Channel: "balance", RechargeUsed: 2000, IncomeUsed: 1800, Status: "holding", CreatedAt: now()}
	raw, _ := json.Marshal(escrow)
	data := map[string]any{}
	json.Unmarshal(raw, &data)
	delete(data, "funding_version")
	if e = put(ctx, tx, "wallet_escrows", id, data); e != nil {
		t.Fatal(e)
	}
	if e = markLegacyFundingEscrows(ctx, tx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return o
}

// TestIntegrationFinancialLegacyBalanceRefund 验证旧混合余额取消/批准退款恢复原分区，重复请求不增发且只标记待核账。
func TestIntegrationFinancialLegacyBalanceRefund(t *testing.T) {
	for _, action := range []string{"cancel", "approve_refund"} {
		t.Run(action, func(t *testing.T) {
			api := newTestAPI(t)
			o := legacyBalanceRefundFixture(t, api)
			if action == "approve_refund" {
				api.action("worker", o.ID, "accept", "", 200)
				api.action("worker", o.ID, "start", "", 200)
				api.action("player", o.ID, "request_refund", "取消本次服务并退还原余额", 200)
			}
			actor := "player"
			if action == "approve_refund" {
				actor = "worker"
			}
			refunded := api.action(actor, o.ID, action, "退回原付款余额", 200)
			api.action(actor, o.ID, action, "重复提交不得再次退款", 409)
			buyer := testWallet(api, "player")
			provider := testWallet(api, "worker")
			if refunded.PaymentStatus != "refunded" || !refunded.FinancialReconciliationRequired || buyer.RechargeCents != 2000 || buyer.AvailableCents != 1800 || provider.PendingCents != 0 || provider.AvailableCents != 0 {
				t.Fatalf("旧混合余额退款不守恒:%+v %+v %+v", refunded, buyer, provider)
			}
			var lots int
			api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM fund_lots").Scan(&lots)
			if lots != 0 {
				t.Fatal("旧余额退款补造了新版可提现来源")
			}
			api.call("player", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 1800, "request_id": "legacy-restored-no-source"}, 409, nil)
			var refunds int
			api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM wallet_entries WHERE data->>'kind'='order_balance_refund' AND data->>'reference_id'=$1", o.ID).Scan(&refunds)
			if refunds != 1 {
				t.Fatalf("余额退款非一次入账:%d", refunds)
			}
		})
	}
}

// TestIntegrationFinancialDamagedBalanceRefund 拒绝新版托管丢失全部或部分来源，也拒绝仅存分配证据的旧JSON降级兼容。
func TestIntegrationFinancialDamagedBalanceRefund(t *testing.T) {
	for _, mode := range []string{"empty", "partial", "missing_version"} {
		t.Run(mode, func(t *testing.T) {
			api := newTestAPI(t)
			testIncome(api, "player", 1800)
			testTopup(api, "player", "balance", "damaged-balance-topup", 2000)
			var o Order
			api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 2}, 200, &o)
			api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{"channel": "balance", "request_id": "damaged-balance-pay"}, 200, nil)
			if mode == "partial" {
				_, e := api.App.DB.Exec(t.Context(), `DELETE FROM fund_lots WHERE id=(SELECT id FROM fund_lots WHERE data->>'account_id'=$1 ORDER BY id LIMIT 1)`, "escrow:"+o.ID)
				if e != nil {
					t.Fatal(e)
				}
			} else {
				_, e := api.App.DB.Exec(t.Context(), "DELETE FROM fund_lots WHERE data->>'account_id'=$1", "escrow:"+o.ID)
				if e != nil {
					t.Fatal(e)
				}
			}
			if mode == "missing_version" {
				if _, e := api.App.DB.Exec(t.Context(), "UPDATE wallet_escrows SET data=data-'funding_version' WHERE id=$1", o.ID); e != nil {
					t.Fatal(e)
				}
				if e := markLegacyFundingEscrows(t.Context(), api.App.DB); e != nil {
					t.Fatal(e)
				}
				escrow, _ := get[WalletEscrow](t.Context(), api.App.DB, "wallet_escrows", o.ID, false)
				if escrow.FundingVersion != 1 || escrow.LegacySourceGap {
					t.Fatal("残留新版分配凭证被错误降级为历史资金")
				}
			}
			api.action("player", o.ID, "cancel", "损坏来源必须拒绝恢复", 409)
			stored, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
			buyer := testWallet(api, "player")
			provider := testWallet(api, "worker")
			if stored.PaymentStatus != "paid" || buyer.RechargeCents != 0 || buyer.AvailableCents != 0 || provider.PendingCents != 3800 {
				t.Fatalf("损坏来源退款错误改账:%+v %+v %+v", stored, buyer, provider)
			}
		})
	}
}
