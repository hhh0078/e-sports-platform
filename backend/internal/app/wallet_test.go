// Package app 验证钱包分区、幂等载荷、订单原分账退款、七天售后冻结以及并发提现不会超支。
package app

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
)

// testWallet 通过公开本人接口读取钱包，验证后端输出而非直接相信数据库夹具。
func testWallet(api *testAPI, user string) Wallet {
	api.t.Helper()
	var wallet Wallet
	api.call(user, "GET", "/wallet", nil, 200, &wallet)
	return wallet
}

// testTopup 通过真实HTTP开发付款通道建立可追踪的充值来源。
func testTopup(api *testAPI, user, purpose, request string, amount int64) WalletTopup {
	api.t.Helper()
	var result struct {
		Payment WalletTopup `json:"payment"`
	}
	api.call(user, "POST", "/wallet/topups", map[string]any{"purpose": purpose, "amount_cents": amount, "request_id": request}, 200, &result)
	return result.Payment
}

// testIncome 仅在独立测试schema中建立有账本的服务收入夹具，生产代码没有此接口。
func testIncome(api *testAPI, user string, amount int64) {
	api.t.Helper()
	ctx := api.t.Context()
	tx, err := api.App.DB.Begin(ctx)
	if err != nil {
		api.t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = lockWalletUsers(ctx, tx, user); err != nil {
		api.t.Fatal(err)
	}
	wallet, err := lockedWallet(ctx, tx, user)
	if err != nil {
		api.t.Fatal(err)
	}
	if err = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "test_service_income", ReferenceID: newID(), AmountCents: amount, AvailableDelta: amount, Note: "测试隔离schema收入夹具"}); err != nil {
		api.t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		api.t.Fatal(err)
	}
}

// TestWalletValidation 检查整数溢出、负金额和售后资格边界，禁止使用订单状态虚构历史收入。
func TestWalletValidation(t *testing.T) {
	for _, pair := range [][2]int64{{math.MaxInt64, 1}, {0, -1}, {1, math.MinInt64}} {
		if _, err := checkedWalletAmount(pair[0], pair[1]); err == nil {
			t.Fatalf("非法余额未拒绝:%v", pair)
		}
	}
	if v, err := checkedWalletAmount(100, -100); err != nil || v != 0 {
		t.Fatal(v, err)
	}
	stamp := time.Now()
	o := Order{Status: "completed", PaymentStatus: "paid", ProviderID: "worker", WalletAccounted: true, CompletedAt: stamp.Add(-walletHoldDuration).Format(time.RFC3339Nano), RefundStatus: "none"}
	if !walletIncomeEligible(o, stamp) {
		t.Fatal("已满售后期应可结算")
	}
	for _, status := range []string{"requested", "escalated", "processing", "rejected"} {
		changed := o
		changed.RefundStatus = status
		if walletIncomeEligible(changed, stamp) {
			t.Fatalf("售后%s仍释放", status)
		}
	}
	o.WalletAccounted = false
	if walletIncomeEligible(o, stamp) {
		t.Fatal("历史无托管订单不应产生收入")
	}
	for _, request := range []string{"", "a", "带中文的请求号", "1234567/", "12345678\n"} {
		if walletRequestID(request) == nil {
			t.Fatalf("非法请求号:%q", request)
		}
	}
}

// TestIntegrationWalletBalanceRefund 验证混合余额优先花充值款、退款精确归还原分账、请求号冲突和账本不可变。
func TestIntegrationWalletBalanceRefund(t *testing.T) {
	api := newTestAPI(t)
	testIncome(api, "player", 3000)
	testTopup(api, "player", "balance", "recharge-0001", 2000)
	testTopup(api, "player", "deposit", "deposit-0001", 4000)
	testTopup(api, "player", "balance", "recharge-0001", 2000)
	api.call("player", "POST", "/wallet/topups", map[string]any{"purpose": "deposit", "amount_cents": 2000, "request_id": "recharge-0001"}, 409, nil)
	testTopup(api, "outsider", "balance", "recharge-0001", 100)
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 2}, 200, &o)
	var paid struct {
		Order Order  `json:"order"`
		Mode  string `json:"mode"`
	}
	payload := map[string]any{"channel": "balance", "request_id": "balance-pay-0001"}
	api.call("player", "POST", "/orders/"+o.ID+"/pay", payload, 200, &paid)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", payload, 200, nil)
	if paid.Mode != "balance" || paid.Order.BalanceRechargeCents != 2000 || paid.Order.BalanceIncomeCents != 1800 || !paid.Order.WalletAccounted {
		t.Fatalf("原支付分账错误:%+v", paid)
	}
	wallet := testWallet(api, "player")
	if wallet.RechargeCents != 0 || wallet.AvailableCents != 1200 || wallet.DepositCents != 4000 {
		t.Fatalf("用途隔离错误:%+v", wallet)
	}
	if wallet = testWallet(api, "worker"); wallet.PendingCents != 3800 || wallet.AvailableCents != 0 {
		t.Fatalf("付款应先托管:%+v", wallet)
	}
	var second Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &second)
	api.call("player", "POST", "/orders/"+second.ID+"/pay", payload, 409, nil)
	api.call("player", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 100, "request_id": "recharge-0001"}, 409, nil)
	api.action("player", o.ID, "cancel", "", 200)
	wallet = testWallet(api, "player")
	if wallet.RechargeCents != 2000 || wallet.AvailableCents != 3000 || wallet.DepositCents != 4000 {
		t.Fatalf("退款未精确还原:%+v", wallet)
	}
	if wallet = testWallet(api, "worker"); wallet.PendingCents != 0 {
		t.Fatalf("退款后仍待结算:%+v", wallet)
	}
	_, err := api.App.DB.Exec(t.Context(), "UPDATE wallet_entries SET data=data WHERE data->>'user_id'='player'")
	if err == nil {
		t.Fatal("资金账本被覆盖")
	}
	code, raw, err := api.request("player", "GET", "/wallet/topups", nil)
	if err != nil || code != 200 {
		t.Fatal(code, err)
	}
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	json.Unmarshal(raw, &envelope)
	for _, p := range envelope.Data {
		if _, ok := p["transaction_id"]; ok {
			t.Fatal("公开响应泄露支付流水")
		}
	}
}

// TestIntegrationWalletSettlement 验证完成后七天、卖家拒绝售后和平台最终裁决影响收入释放，重复读取不会重复结算。
func TestIntegrationWalletSettlement(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	api.action("worker", o.ID, "accept", "", 200)
	api.action("worker", o.ID, "start", "", 200)
	api.action("worker", o.ID, "deliver", "目标已经完成", 200)
	o = api.action("player", o.ID, "confirm", "", 200)
	if v := testWallet(api, "worker"); v.PendingCents != 3800 || v.AvailableCents != 0 {
		t.Fatalf("未满售后期释放:%+v", v)
	}
	o.CompletedAt = time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	o.RefundStatus = "requested"
	if err := put(t.Context(), api.App.DB, "orders", o.ID, o); err != nil {
		t.Fatal(err)
	}
	if v := testWallet(api, "worker"); v.AvailableCents != 0 {
		t.Fatal("售后中仍释放")
	}
	o.RefundStatus = "rejected"
	put(t.Context(), api.App.DB, "orders", o.ID, o)
	if v := testWallet(api, "worker"); v.AvailableCents != 0 {
		t.Fatal("未最终裁决的拒绝退款仍释放")
	}
	o.RefundStatus = "escalated"
	put(t.Context(), api.App.DB, "orders", o.ID, o)
	api.call("admin", "POST", "/admin/orders/"+o.ID+"/resolve", map[string]string{"resolution": "complete", "note": "完整证据确认服务交付"}, 200, &o)
	if !o.RefundFinal {
		t.Fatal("最终裁决未记录")
	}
	api.action("player", o.ID, "escalate", "再次申请", 409)
	api.action("player", o.ID, "request_refund", "再次退款", 409)
	for range 2 {
		if v := testWallet(api, "worker"); v.AvailableCents != 3800 || v.PendingCents != 0 {
			t.Fatalf("收入未只释放一次:%+v", v)
		}
	}
	historical := o
	historical.ID = newID()
	historical.OrderNo = "HISTORY" + historical.ID
	historical.WalletAccounted = false
	historical.AmountCents = 99999
	put(t.Context(), api.App.DB, "orders", historical.ID, historical)
	if v := testWallet(api, "worker"); v.AvailableCents != 3800 {
		t.Fatal("历史订单虚增余额")
	}
}

// TestIntegrationWalletConcurrentWithdrawals 验证并发冻结不超提、跨账号访问、失败和拒绝解冻以及成功重复回放。
func TestIntegrationWalletConcurrentWithdrawals(t *testing.T) {
	api := newTestAPI(t)
	testIncome(api, "worker", 1000)
	testTopup(api, "worker", "balance", "withdraw-recharge-1", 8000)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := []WalletWithdrawal{}
	for i := range 12 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			code, raw, err := api.request("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 700, "request_id": fmt.Sprintf("withdraw-%08d", index)})
			if err != nil {
				t.Error(err)
				return
			}
			if code == 200 {
				var result struct {
					Data WalletWithdrawal `json:"data"`
				}
				json.Unmarshal(raw, &result)
				mu.Lock()
				success = append(success, result.Data)
				mu.Unlock()
			} else if code != 409 {
				t.Errorf("并发提现异常:%d %s", code, raw)
			}
		}(i)
	}
	wg.Wait()
	if len(success) != 1 {
		t.Fatalf("并发冻结发生超提:%d", len(success))
	}
	v := success[0]
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 300 || wallet.WithdrawalFrozenCents != 700 || wallet.RechargeCents != 8000 {
		t.Fatalf("资金冻结错误:%+v", wallet)
	}
	api.call("outsider", "POST", "/wallet/withdrawals/"+v.ID+"/sync", map[string]any{}, 403, nil)
	api.call("worker", "POST", "/admin/wallet/withdrawals/"+v.ID+"/execute", map[string]string{"dev_outcome": "success"}, 403, nil)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+v.ID+"/execute", map[string]string{"dev_outcome": "failed"}, 200, nil)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+v.ID+"/execute", map[string]string{"dev_outcome": "failed"}, 200, nil)
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 1000 || wallet.WithdrawalFrozenCents != 0 {
		t.Fatalf("失败未解冻:%+v", wallet)
	}
	var reject WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 100, "request_id": "reject-withdraw-0001"}, 200, &reject)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+reject.ID+"/reject", map[string]string{"reason": "测试审核资料不符"}, 200, nil)
	var paid WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 900, "request_id": "success-withdraw-0001"}, 200, &paid)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+paid.ID+"/execute", map[string]string{"dev_outcome": "success"}, 200, nil)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+paid.ID+"/execute", map[string]string{"dev_outcome": "success"}, 200, nil)
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 100 || wallet.WithdrawalFrozenCents != 0 || wallet.RechargeCents != 8000 {
		t.Fatalf("成功重复扣款或动用充值:%+v", wallet)
	}
}

// TestIntegrationWalletDepositRefund 验证多笔押金部分退款和买卖双方未完成订单阻塞，押金不会转成可用余额。
func TestIntegrationWalletDepositRefund(t *testing.T) {
	api := newTestAPI(t)
	testTopup(api, "outsider", "deposit", "deposit-source-0001", 200)
	testTopup(api, "outsider", "deposit", "deposit-source-0002", 300)
	var refund WalletDepositRefund
	api.call("outsider", "POST", "/wallet/deposit-refunds", map[string]any{"amount_cents": 400, "request_id": "deposit-refund-0001"}, 200, &refund)
	if len(refund.Parts) != 2 || refund.Status != "success" {
		t.Fatalf("多笔退款未完成:%+v", refund)
	}
	api.call("outsider", "POST", "/wallet/deposit-refunds", map[string]any{"amount_cents": 400, "request_id": "deposit-refund-0001"}, 200, nil)
	api.call("outsider", "POST", "/wallet/deposit-refunds", map[string]any{"amount_cents": 100, "request_id": "deposit-refund-0001"}, 409, nil)
	if wallet := testWallet(api, "outsider"); wallet.DepositCents != 100 || wallet.DepositRefundingCents != 0 || wallet.RechargeCents != 0 || wallet.AvailableCents != 0 {
		t.Fatalf("押金退款用途错误:%+v", wallet)
	}
	testTopup(api, "player", "deposit", "blocked-buyer-0001", 100)
	testTopup(api, "worker", "deposit", "blocked-provider-0001", 100)
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	for _, user := range []string{"player", "worker"} {
		api.call(user, "POST", "/wallet/deposit-refunds", map[string]any{"request_id": "blocked-refund-0001"}, 409, nil)
	}
	api.action("player", order.ID, "cancel", "", 200)
	api.call("worker", "POST", "/wallet/deposit-refunds", map[string]any{"request_id": "blocked-refund-0001"}, 200, nil)
}

// TestIntegrationWalletProductionRejectsDevFunds 确认本地模拟余额不能通过切换配置变成生产可提现资金。
func TestIntegrationWalletProductionRejectsDevFunds(t *testing.T) {
	api := newTestAPI(t)
	if err := guardProductionWalletData(t.Context(), api.App.DB); err != nil {
		t.Fatal("空钱包不应阻止新生产库启动")
	}
	testTopup(api, "player", "balance", "dev-funds-isolation-0001", 100)
	if err := guardProductionWalletData(t.Context(), api.App.DB); err == nil {
		t.Fatal("DEV充值被允许进入真实资金环境")
	}
	cfg := api.App.Config
	cfg.DevMode = false
	cfg.Env = "production"
	app, err := New(t.Context(), cfg)
	if err == nil {
		app.Close()
		t.Fatal("生产启动接受了模拟资金")
	}
}
