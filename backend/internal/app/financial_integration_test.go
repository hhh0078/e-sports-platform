// Package app 验证佣金快照、三类押金隔离及递归来源提现，所有数据位于独立临时测试schema。
package app

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// matureFinancialOrder 通过交易状态机交付验收，仅将测试订单时间移到售后结束以验证真实结算入口。
func matureFinancialOrder(api *testAPI, o Order, provider, buyer string) Order {
	api.t.Helper()
	api.action(provider, o.ID, "accept", "", 200)
	api.action(provider, o.ID, "start", "", 200)
	api.action(provider, o.ID, "deliver", "已经完成约定服务内容", 200)
	o = api.action(buyer, o.ID, "confirm", "", 200)
	o.CompletedAt = time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	if e := put(api.t.Context(), api.App.DB, "orders", o.ID, o); e != nil {
		api.t.Fatal(e)
	}
	return o
}

// financialOrder 创建指定价格快照订单并由指定买家支付，不从客户端传入价格。
func financialOrder(api *testAPI, buyer, service, channel, request string) Order {
	api.t.Helper()
	var o Order
	api.call(buyer, "POST", "/orders", map[string]any{"service_id": service, "quantity": 1}, 200, &o)
	body := map[string]any{}
	if channel == "balance" {
		body["channel"] = channel
		body["request_id"] = request
	}
	api.call(buyer, "POST", "/orders/"+o.ID+"/pay", body, 200, nil)
	return o
}

// TestIntegrationFinancialCommissionSnapshots 验证两档平台佣金、组织独立钱包、整数余数及旧单快照稳定。
func TestIntegrationFinancialCommissionSnapshots(t *testing.T) {
	api := newTestAPI(t)
	api.call("worker", "PATCH", "/admin/financial-settings", FinancialSettings{}, 403, nil)
	settings := FinancialSettings{FreePlatformRateBPS: 200, ClubPlatformRateBPS: 500}
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	club, _ := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	club.CommissionRateBPS = 1000
	put(t.Context(), api.App.DB, "clubs", club.ID, club)
	s, _ := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	s.PriceCents = 1001
	put(t.Context(), api.App.DB, "services", s.ID, s)
	o := financialOrder(api, "player", s.ID, "dev", "")
	if o.Settlement.PlatformFeeCents != 50 || o.Settlement.ClubFeeCents != 100 || o.Settlement.ProviderIncomeCents != 851 {
		t.Fatalf("分账不守恒:%+v", o.Settlement)
	}
	settings.ClubPlatformRateBPS = 800
	api.call("admin", "PATCH", "/admin/financial-settings", settings, 200, nil)
	matureFinancialOrder(api, o, "worker", "player")
	wallet := testWallet(api, "worker")
	if wallet.AvailableCents != 851 || wallet.PendingCents != 0 {
		t.Fatalf("打手错误收入:%+v", wallet)
	}
	var cwallet map[string]any
	api.call("clubowner", "GET", "/clubs/shanhai/wallet", nil, 200, &cwallet)
	if cwallet["available_cents"] != float64(100) {
		t.Fatalf("组织佣金未独立:%v", cwallet)
	}
	api.call("worker", "GET", "/clubs/shanhai/wallet", nil, 403, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/withdrawals", map[string]any{"amount_cents": 100, "request_id": "club-commission-withdraw"}, 200, nil)
	if testWallet(api, "clubowner").AvailableCents != 0 {
		t.Fatal("组织佣金污染个人余额")
	}
	s.ClubID = ""
	put(t.Context(), api.App.DB, "services", s.ID, s)
	var free Order
	api.call("outsider", "POST", "/orders", map[string]any{"service_id": s.ID, "quantity": 1}, 200, &free)
	if free.Settlement.PlatformRateBPS != 200 || free.Settlement.ClubFeeCents != 0 {
		t.Fatal("自由打手未使用独立费率")
	}
}

// TestIntegrationFinancialFiveOrderAudit 验证五单收入逐笔溯源、退款恢复来源、每日限制及审核权限。
func TestIntegrationFinancialFiveOrderAudit(t *testing.T) {
	api := newTestAPI(t)
	s, _ := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	s.PriceCents = 2000
	put(t.Context(), api.App.DB, "services", s.ID, s)
	testTopup(api, "player", "balance", "five-orders-recharge", 10000)
	for i := 0; i < 5; i++ {
		o := financialOrder(api, "player", s.ID, "balance", fmt.Sprintf("five-order-pay-%d", i))
		matureFinancialOrder(api, o, "worker", "player")
	}
	if testWallet(api, "worker").AvailableCents != 10000 {
		t.Fatal("五单结算错误")
	}
	var v WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 10000, "request_id": "five-orders-withdraw"}, 200, &v)
	var report struct {
		Passed  bool               `json:"passed"`
		Sources []withdrawalSource `json:"sources"`
	}
	api.call("worker", "GET", "/admin/wallet/withdrawals/"+v.ID+"/audit", nil, 403, nil)
	api.call("admin", "GET", "/admin/wallet/withdrawals/"+v.ID+"/audit", nil, 200, &report)
	if !report.Passed || len(report.Sources) != 5 {
		t.Fatalf("五单来源未完整核验:%+v", report)
	}
	for _, s := range report.Sources {
		if s.OrderID == "" || s.PaymentID == "" || s.AmountCents != 2000 {
			t.Fatalf("资金来源缺凭证:%+v", s)
		}
	}
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 1, "request_id": "second-daily-withdraw"}, 409, nil)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+v.ID+"/reject", map[string]string{"reason": "测试恢复来源"}, 200, nil)
	var retry WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 10000, "request_id": "five-orders-withdraw-retry"}, 200, &retry)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+retry.ID+"/execute", map[string]string{"dev_outcome": "success"}, 200, nil)
	if testWallet(api, "worker").AvailableCents != 0 {
		t.Fatal("提现未扣除对应来源")
	}
}

// TestIntegrationFinancialRecursiveBalance 验证混合充值收入支付、退款保持分区及收入再次消费后仍追到原收款根。
func TestIntegrationFinancialRecursiveBalance(t *testing.T) {
	api := newTestAPI(t)
	testIncome(api, "player", 1800)
	testTopup(api, "player", "balance", "mixed-real-source-1", 1000)
	testTopup(api, "player", "balance", "mixed-real-source-2", 1000)
	var first Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 2}, 200, &first)
	api.call("player", "POST", "/orders/"+first.ID+"/pay", map[string]any{"channel": "balance", "request_id": "mixed-order-source-pay"}, 200, nil)
	api.action("player", first.ID, "cancel", "取消本次服务", 200)
	wallet := testWallet(api, "player")
	if wallet.RechargeCents != 2000 || wallet.AvailableCents != 1800 {
		t.Fatalf("混合退款改变资金用途:%+v", wallet)
	}
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 2}, 200, &o)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{"channel": "balance", "request_id": "mixed-second-source-pay"}, 200, nil)
	matureFinancialOrder(api, o, "worker", "player")
	testWallet(api, "worker")
	s, _ := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	s.ID = "recursive-outsider-service"
	s.ProviderID = "outsider"
	s.ClubID = ""
	s.PriceCents = 3800
	put(t.Context(), api.App.DB, "services", s.ID, s)
	next := financialOrder(api, "worker", s.ID, "balance", "income-repurchase-order")
	matureFinancialOrder(api, next, "outsider", "worker")
	testWallet(api, "outsider")
	var v WalletWithdrawal
	api.call("outsider", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 3800, "request_id": "recursive-income-withdraw"}, 200, &v)
	sources, issues, e := auditWithdrawalSources(t.Context(), api.App.DB, v, true)
	if e != nil || len(issues) > 0 || len(sources) != 3 {
		t.Fatalf("递归来源核验失败:%v %v %+v", e, issues, sources)
	}
	_, issues, e = auditWithdrawalSources(t.Context(), api.App.DB, v, false)
	if e != nil || len(issues) == 0 {
		t.Fatal("模拟收款根进入真实出款")
	}
}

// TestIntegrationFinancialLegacyAndConcurrent 拒绝无支付来源的历史余额，并发申请最多一笔占用来源与日次数。
func TestIntegrationFinancialLegacyAndConcurrent(t *testing.T) {
	api := newTestAPI(t)
	tx, _ := api.App.DB.Begin(t.Context())
	lockWalletUsers(t.Context(), tx, "outsider")
	wallet, _ := lockedWallet(t.Context(), tx, "outsider")
	appendWalletEntry(t.Context(), tx, &wallet, WalletEntry{Kind: "historical_unknown", ReferenceID: "unknown", AvailableDelta: 100, AmountCents: 100, Note: "缺来源历史数据"})
	tx.Commit(t.Context())
	api.call("outsider", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 100, "request_id": "unknown-source-refused"}, 409, nil)
	testIncome(api, "worker", 10000)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, _, e := api.request("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 100, "request_id": fmt.Sprintf("daily-concurrent-%03d", i)})
			if e != nil {
				t.Error(e)
			}
			if code == 200 {
				mu.Lock()
				success++
				mu.Unlock()
			} else if code != 409 {
				t.Errorf("并发异常%d", code)
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("每日并发限制失效:%d", success)
	}
}

// TestIntegrationFinancialClubDeposits 验证创建押金、历史补缴门槛、认证阻塞退款与两类组织押金隔离。
func TestIntegrationFinancialClubDeposits(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{ClubCreationDepositCents: 50}, 200, nil)
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 409, nil)
	var draft Club
	api.call("outsider", "POST", "/clubs", map[string]any{"name": "押金测试俱乐部", "description": "独立组织平台押金流程测试介绍", "game_ids": []string{"delta"}}, 200, &draft)
	if draft.Status != "pending_deposit" {
		t.Fatal("未缴创建押金即送审")
	}
	api.call("admin", "PATCH", "/admin/clubs/"+draft.ID, map[string]any{"status": "active", "verified": true}, 409, nil)
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": draft.ID, "amount_cents": 50, "request_id": "new-club-platform-deposit"}, 200, nil)
	api.call("admin", "PATCH", "/admin/clubs/"+draft.ID, map[string]any{"status": "active", "verified": true}, 200, nil)
	api.call("clubowner", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": "shanhai", "amount_cents": 50, "request_id": "legacy-club-platform-deposit"}, 200, nil)
	api.call("clubowner", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "club_platform", "club_id": "shanhai", "request_id": "club-refund-active-members"}, 409, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/worker", map[string]any{"status": "removed"}, 200, nil)
	api.call("clubowner", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "club_platform", "club_id": "shanhai", "request_id": "club-refund-zero-members"}, 200, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/worker", map[string]any{"status": "active"}, 409, nil)
	c, _ := get[Club](t.Context(), api.App.DB, "clubs", draft.ID, false)
	c.DepositCents = 25
	put(t.Context(), api.App.DB, "clubs", c.ID, c)
	api.call("worker", "POST", "/clubs/"+c.ID+"/join", map[string]any{"requirements_accepted": true}, 200, nil)
	api.call("outsider", "POST", "/clubs/"+c.ID+"/members/worker", map[string]any{"status": "active"}, 409, nil)
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "member_club", "club_id": c.ID, "amount_cents": 25, "request_id": "member-club-precise-deposit"}, 200, nil)
	api.call("outsider", "POST", "/clubs/"+c.ID+"/members/worker", map[string]any{"status": "active"}, 200, nil)
	api.call("worker", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "member_club", "club_id": c.ID, "request_id": "member-refund-still-active"}, 409, nil)
	api.call("worker", "POST", "/clubs/"+c.ID+"/leave", map[string]any{}, 200, nil)
	api.call("worker", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "member_club", "club_id": c.ID, "request_id": "member-refund-left-club"}, 200, nil)
	var detail map[string]json.RawMessage
	api.call("outsider", "GET", "/clubs/"+c.ID+"/wallet", nil, 200, &detail)
	var available int64
	json.Unmarshal(detail["available_cents"], &available)
	if available != 0 {
		t.Fatal("成员押金成为组织可提现收入")
	}
}

// TestIntegrationFinancialClubDraftIdempotency 验证丢失创建响应后同号恢复草稿，改写载荷不能重复建组织。
func TestIntegrationFinancialClubDraftIdempotency(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{ClubCreationDepositCents: 77}, 200, nil)
	body := map[string]any{"request_id": "club-draft-idempotent-0001", "name": "幂等创建俱乐部", "description": "用于验证创建请求丢失后的原草稿恢复流程", "game_ids": []string{"delta"}}
	var first, second Club
	api.call("outsider", "POST", "/clubs", body, 200, &first)
	api.call("outsider", "POST", "/clubs", body, 200, &second)
	if first.ID != second.ID || first.CreationDepositRequiredCents != 77 || first.Status != "pending_deposit" {
		t.Fatalf("创建门槛或幂等异常:%+v %+v", first, second)
	}
	body["name"] = "修改同一幂等请求"
	api.call("outsider", "POST", "/clubs", body, 409, nil)
	api.call("outsider", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": first.ID, "amount_cents": 77, "request_id": "idempotent-club-deposit-pay"}, 200, nil)
	api.call("admin", "PATCH", "/admin/clubs/"+first.ID, map[string]any{"status": "rejected", "verified": false}, 200, nil)
	api.call("outsider", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "club_platform", "club_id": first.ID, "request_id": "rejected-club-deposit-refund"}, 200, nil)
	wallet, _ := accountBalance(t.Context(), api.App.DB, clubDepositAccount(first.ID))
	if wallet.DepositCents != 0 || wallet.DepositRefundingCents != 0 {
		t.Fatal("审核拒绝的组织押金无法退清")
	}
}

// TestIntegrationFinancialAuditMerchantMismatch 验证真实来源不能跨商户出款，以及不可变凭证和来源容量保护。
func TestIntegrationFinancialAuditMerchantMismatch(t *testing.T) {
	api, c, _ := productionWalletAPI(t)
	testIncome(api, "worker", 1000)
	settings := WalletSettings{WithdrawalEnabled: true, TransferSceneID: "approved-test-scene", TransferNotifyURL: "https://example.com/api/payments/wechat/transfer-notify", TransferRemark: "服务收入提现", TransferSceneReportInfos: []WalletSceneReport{{InfoType: "岗位类型", InfoContent: "自动测试"}}}
	api.call("admin", "PATCH", "/admin/wallet-settings", settings, 200, nil)
	var v WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 100, "request_id": "merchant-audit-source"}, 200, &v)
	stored, _ := get[WalletWithdrawal](t.Context(), api.App.DB, "wallet_withdrawals", v.ID, false)
	_, issues, e := auditWithdrawalSources(t.Context(), api.App.DB, stored, false)
	if e != nil || len(issues) > 0 {
		t.Fatalf("原商户来源未通过:%v %v", e, issues)
	}
	stored.MerchantID = c.MerchantID + "changed"
	_, issues, e = auditWithdrawalSources(t.Context(), api.App.DB, stored, false)
	if e != nil || len(issues) == 0 {
		t.Fatal("跨商户来源仍通过")
	}
	lots, _ := list[FundingLot](t.Context(), api.App.DB, "fund_lots", "WHERE data->>'account_id'=$1", "withdrawal:"+v.ID)
	if len(lots) == 0 {
		t.Fatal("提现无资金来源")
	}
	lot := lots[0]
	lot.AmountCents++
	if e = put(t.Context(), api.App.DB, "fund_lots", lot.ID, lot); e == nil {
		t.Fatal("来源批次原始金额可被改写")
	}
	receipt, _ := get[FundingReceipt](t.Context(), api.App.DB, "fund_receipts", lots[0].ReceiptID, false)
	receipt.AmountCents++
	if e = put(t.Context(), api.App.DB, "fund_receipts", receipt.ID, receipt); e == nil {
		t.Fatal("原始收款凭证可被覆盖")
	}
}
