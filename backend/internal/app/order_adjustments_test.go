// Package app 验证订单加减价的双方权限、原路分账、后续全退和并发隔离，测试仅使用临时数据库schema。
package app

import (
	"encoding/json"
	"net/url"
	"sync"
	"testing"
	"time"
)

// proposeTestAdjustment 通过正式HTTP入口创建指定差额的协商方案。
func proposeTestAdjustment(api *testAPI, o Order, user, kind, request string, amount int64) OrderAdjustment {
	api.t.Helper()
	var v OrderAdjustment
	api.call(user, "POST", "/orders/"+o.ID+"/adjustments", map[string]any{"kind": kind, "amount_cents": amount, "reason": "双方协商调整游戏服务费用", "request_id": request}, 200, &v)
	return v
}

// respondTestAdjustment 通过正式入口回应调价并返回服务端最终状态。
func respondTestAdjustment(api *testAPI, v OrderAdjustment, user, action string, status int) OrderAdjustment {
	api.t.Helper()
	var out OrderAdjustment
	if status != 200 {
		api.call(user, "POST", "/orders/"+v.OrderID+"/adjustments/"+v.ID+"/respond", map[string]string{"action": action}, status, nil)
		return out
	}
	api.call(user, "POST", "/orders/"+v.OrderID+"/adjustments/"+v.ID+"/respond", map[string]string{"action": action}, status, &out)
	return out
}

// payTestAdjustment 支付已确认的加价并重新获取订单当前净价。
func payTestAdjustment(api *testAPI, v OrderAdjustment, channel, request string) Order {
	api.t.Helper()
	api.call("player", "POST", "/orders/"+v.OrderID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": channel, "request_id": request}, 200, nil)
	var o Order
	api.call("player", "GET", "/orders/"+v.OrderID, nil, 200, &o)
	return o
}

// TestIntegrationOrderAdjustmentsBalanceAndFullRefund 验证混合余额加价、跨两原单减价与最终全退都严格恢复原资金用途。
func TestIntegrationOrderAdjustmentsBalanceAndFullRefund(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{ClubPlatformRateBPS: 500}, 200, nil)
	c, _ := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	c.CommissionRateBPS = 1000
	if e := put(t.Context(), api.App.DB, "clubs", c.ID, c); e != nil {
		t.Fatal(e)
	}
	s, _ := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	s.PriceCents = 1001
	if e := put(t.Context(), api.App.DB, "services", s.ID, s); e != nil {
		t.Fatal(e)
	}
	testIncome(api, "player", 600)
	testTopup(api, "player", "balance", "adjust-mixed-recharge", 1000)
	o := financialOrder(api, "player", s.ID, "balance", "adjust-initial-payment")
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-increase-first", 101)
	respondTestAdjustment(api, v, "player", "accept", 200)
	o = payTestAdjustment(api, v, "balance", "adjust-supplement-pay")
	if o.AmountCents != 1102 || o.OriginalAmountCents != 1001 || o.PaidTotalCents != 1102 || o.Settlement.ProviderIncomeCents != 937 {
		t.Fatalf("补款分账错误:%+v", o)
	}
	// 后续配置变化不能追改这笔订单调价时使用的冻结费率。
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{ClubPlatformRateBPS: 800}, 200, nil)
	v = proposeTestAdjustment(api, o, "player", "decrease", "adjust-decrease-first", 303)
	v = respondTestAdjustment(api, v, "worker", "accept", 200)
	if v.Status != "completed" {
		t.Fatalf("余额减价未完成:%+v", v)
	}
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	if o.AmountCents != 799 || o.PriceRefundedCents != 303 || o.Settlement.PlatformRateBPS != 500 || o.Settlement.ClubRateBPS != 1000 || o.Settlement.ProviderIncomeCents != 681 {
		t.Fatalf("减价分账错误:%+v", o)
	}
	wallet := testWallet(api, "player")
	if wallet.RechargeCents != 202 || wallet.AvailableCents != 599 {
		t.Fatalf("部分退款改变原资金分区:%+v", wallet)
	}
	worker := testWallet(api, "worker")
	if worker.PendingCents != 681 {
		t.Fatalf("净收入分录错误:%+v", worker)
	}
	api.action("worker", o.ID, "accept", "", 200)
	api.action("player", o.ID, "request_refund", "双方同意取消剩余服务", 200)
	o = api.action("worker", o.ID, "approve_refund", "同意全额退还剩余款", 200)
	if o.Status != "refunded" || o.PaymentStatus != "refunded" {
		t.Fatalf("调整后全退未关闭:%+v", o)
	}
	wallet = testWallet(api, "player")
	if wallet.RechargeCents != 1000 || wallet.AvailableCents != 600 {
		t.Fatalf("最终全退未恢复原余额:%+v", wallet)
	}
	if testWallet(api, "worker").PendingCents != 0 {
		t.Fatal("全退未反冲全部净收入")
	}
	var cwallet map[string]any
	api.call("clubowner", "GET", "/clubs/shanhai/wallet", nil, 200, &cwallet)
	if cwallet["pending_cents"] != float64(0) {
		t.Fatal("全退未反冲组织佣金")
	}
	parts, e := list[OrderPartRefund](t.Context(), api.App.DB, "order_refunds", "WHERE data->>'order_id'=$1", o.ID)
	if e != nil {
		t.Fatal(e)
	}
	var total int64
	for _, p := range parts {
		if p.Status != "success" {
			t.Fatal("存在未完成退款")
		}
		total += p.AmountCents
	}
	if total != 1102 {
		t.Fatalf("累计退款不等于累计实付:%d", total)
	}
}

// TestIntegrationOrderAdjustmentsPermissions 验证四身份只能查看所属订单，管理员和负责人不能代替交易双方同意。
func TestIntegrationOrderAdjustmentsPermissions(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	original := o.AmountCents
	body := map[string]any{"kind": "increase", "amount_cents": 200, "reason": "协商增加新的服务内容", "request_id": "adjust-permissions-request"}
	for _, user := range []string{"player", "admin", "clubowner", "outsider"} {
		api.call(user, "POST", "/orders/"+o.ID+"/adjustments", body, 403, nil)
	}
	var v OrderAdjustment
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", body, 200, &v)
	var replay OrderAdjustment
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", body, 200, &replay)
	if replay.ID != v.ID {
		t.Fatal("幂等申请生成第二方案")
	}
	body["amount_cents"] = 201
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", body, 409, nil)
	for _, user := range []string{"player", "worker", "admin", "clubowner"} {
		api.call(user, "GET", "/orders/"+o.ID+"/adjustments", nil, 200, nil)
	}
	api.call("outsider", "GET", "/orders/"+o.ID+"/adjustments", nil, 403, nil)
	for _, user := range []string{"worker", "admin", "clubowner", "outsider"} {
		respondTestAdjustment(api, v, user, "accept", 403)
	}
	api.action("player", o.ID, "cancel", "", 409)
	api.action("worker", o.ID, "accept", "", 409)
	v = respondTestAdjustment(api, v, "player", "accept", 200)
	if v.Status != "awaiting_payment" || !v.BuyerAgreed || !v.ProviderAgreed {
		t.Fatal("双方同意状态缺失")
	}
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": "wechat"}, 403, nil)
	o = payTestAdjustment(api, v, "wechat", "")
	if o.ActiveAdjustmentID != "" || o.AmountCents != original+200 {
		t.Fatalf("补款净价异常:%+v", o)
	}
	api.call("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": "wechat"}, 200, nil)
	if testWallet(api, "worker").PendingCents != original+200 {
		t.Fatal("补款重放重复收入")
	}
}

// TestIntegrationOrderAdjustmentsConcurrent 验证并发双方同意和重复支付只产生一笔原单及一份收入。
func TestIntegrationOrderAdjustmentsConcurrent(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-concurrent-request", 1)
	var wg sync.WaitGroup
	codes := make(chan int, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _, e := api.request("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/respond", map[string]string{"action": "accept"})
			if e != nil {
				codes <- 0
			} else {
				codes <- status
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _, e := api.request("player", "POST", "/orders/"+o.ID+"/adjustments/"+v.ID+"/pay", map[string]string{"channel": "wechat"})
			if e != nil {
				codes <- 0
			} else {
				codes <- status
			}
		}()
	}
	wg.Wait()
	close(codes)
	for status := range codes {
		if status != 200 {
			t.Fatalf("并发幂等出现错误:%d", status)
		}
	}
	var result Order
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &result)
	if result.AmountCents != o.AmountCents+1 || testWallet(api, "worker").PendingCents != o.AmountCents+1 {
		t.Fatal("并发重复补款或收入")
	}
	payments, e := list[OrderPayment](t.Context(), api.App.DB, "order_payments", "WHERE data->>'order_id'=$1", o.ID)
	if e != nil || len(payments) != 2 {
		t.Fatalf("补款原单重复:%v %d", e, len(payments))
	}
}

// TestIntegrationOrderAdjustmentsExpiryAndSettlement 验证方案过期、售后期减价与结算后禁止改资金。
func TestIntegrationOrderAdjustmentsExpiryAndSettlement(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-expiry-request", 10)
	v.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if e := put(t.Context(), api.App.DB, "order_adjustments", v.ID, v); e != nil {
		t.Fatal(e)
	}
	var list []OrderAdjustment
	api.call("player", "GET", "/orders/"+o.ID+"/adjustments", nil, 200, &list)
	if len(list) != 1 || list[0].Status != "expired" {
		t.Fatal("未出网方案没有过期")
	}
	api.action("worker", o.ID, "accept", "", 200)
	api.action("worker", o.ID, "start", "", 200)
	api.action("worker", o.ID, "deliver", "完成全部游戏任务", 200)
	o = api.action("player", o.ID, "confirm", "", 200)
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", map[string]any{"kind": "increase", "amount_cents": 1, "reason": "完成后加价不能发起", "request_id": "adjust-complete-increase"}, 409, nil)
	v = proposeTestAdjustment(api, o, "worker", "decrease", "adjust-complete-decrease", 100)
	respondTestAdjustment(api, v, "player", "accept", 200)
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	o.CompletedAt = time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	if e := put(t.Context(), api.App.DB, "orders", o.ID, o); e != nil {
		t.Fatal(e)
	}
	if testWallet(api, "worker").AvailableCents != o.AmountCents {
		t.Fatal("净价未正确释放可提现资金")
	}
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", map[string]any{"kind": "decrease", "amount_cents": 1, "reason": "结算后不能自动改价", "request_id": "adjust-released-decrease"}, 409, nil)
	var withdrawal WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": o.AmountCents, "request_id": "adjusted-order-withdraw"}, 200, &withdrawal)
	var auditReport struct {
		Passed  bool               `json:"passed"`
		Sources []withdrawalSource `json:"sources"`
	}
	api.call("admin", "GET", "/admin/wallet/withdrawals/"+withdrawal.ID+"/audit", nil, 200, &auditReport)
	if !auditReport.Passed || len(auditReport.Sources) == 0 {
		t.Fatal("调价后提现来源没有核验")
	}
}

// TestIntegrationOrderAdjustmentsAnalytics 验证补款与多笔退款以各自发生时间计入交易曲线。
func TestIntegrationOrderAdjustmentsAnalytics(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-analytics-increase", 500)
	respondTestAdjustment(api, v, "player", "accept", 200)
	o = payTestAdjustment(api, v, "wechat", "")
	v = proposeTestAdjustment(api, o, "player", "decrease", "adjust-analytics-decrease", 600)
	respondTestAdjustment(api, v, "worker", "accept", 200)
	var result AnalyticsResult
	api.call("admin", "GET", "/admin/analytics", nil, 200, &result)
	if result.Summary.GMVCents != o.PaidTotalCents || result.Summary.RefundCents != 600 || result.Summary.PaidOrderCount != 1 || result.Summary.RefundedOrderCount != 1 {
		raw, _ := json.Marshal(result)
		t.Fatalf("调价统计异常:%s", raw)
	}
	api.action("player", o.ID, "cancel", "全部取消", 200)
	api.call("admin", "GET", "/admin/analytics", nil, 200, &result)
	if result.Summary.RefundCents != o.PaidTotalCents {
		t.Fatalf("最终全退重复或遗漏:%+v", result.Summary)
	}
	// 原单事实受数据库保护，禁止为了通过来源审计直接修改原付款金额。
	payments, _ := list[OrderPayment](t.Context(), api.App.DB, "order_payments", "WHERE data->>'order_id'=$1", o.ID)
	p := payments[0]
	p.AmountCents++
	if e := put(t.Context(), api.App.DB, "order_payments", p.ID, p); e == nil {
		t.Fatal("数据库允许篡改成功原付款")
	}
}

// TestIntegrationOrderAdjustmentsSupplementWithdrawal 验证补款与减价之后个人及组织提现仍能追溯每个真实支付根。
func TestIntegrationOrderAdjustmentsSupplementWithdrawal(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{ClubPlatformRateBPS: 500}, 200, nil)
	c, _ := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	c.CommissionRateBPS = 1000
	if e := put(t.Context(), api.App.DB, "clubs", c.ID, c); e != nil {
		t.Fatal(e)
	}
	o := api.newOrder()
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-withdraw-increase", 501)
	respondTestAdjustment(api, v, "player", "accept", 200)
	o = payTestAdjustment(api, v, "wechat", "")
	v = proposeTestAdjustment(api, o, "worker", "decrease", "adjust-withdraw-decrease", 101)
	respondTestAdjustment(api, v, "player", "accept", 200)
	api.call("player", "GET", "/orders/"+o.ID, nil, 200, &o)
	matureFinancialOrder(api, o, "worker", "player")
	if testWallet(api, "worker").AvailableCents != o.Settlement.ProviderIncomeCents {
		t.Fatal("加减价净收入结算错误")
	}
	for _, entry := range []struct {
		user, path string
		amount     int64
	}{{"worker", "/wallet/withdrawals", o.Settlement.ProviderIncomeCents}, {"clubowner", "/clubs/shanhai/withdrawals", o.Settlement.ClubFeeCents}} {
		var withdrawal WalletWithdrawal
		api.call(entry.user, "POST", entry.path, map[string]any{"amount_cents": entry.amount, "request_id": "adjust-source-" + entry.user}, 200, &withdrawal)
		var report struct {
			Passed  bool               `json:"passed"`
			Sources []withdrawalSource `json:"sources"`
		}
		api.call("admin", "GET", "/admin/wallet/withdrawals/"+withdrawal.ID+"/audit", nil, 200, &report)
		if !report.Passed || len(report.Sources) == 0 {
			t.Fatalf("%s提现来源核验失败:%+v", entry.user, report)
		}
	}
}

// TestIntegrationOrderAdjustmentsAnalyticsSingleConnection 验证统计读取已释放外层游标，单连接池不会被分笔核验自锁。
func TestIntegrationOrderAdjustmentsAnalyticsSingleConnection(t *testing.T) {
	cfg := LoadConfig()
	databaseURL, e := url.Parse(cfg.DatabaseURL)
	if e != nil {
		t.Fatal(e)
	}
	query := databaseURL.Query()
	query.Set("pool_max_conns", "1")
	databaseURL.RawQuery = query.Encode()
	t.Setenv("DATABASE_URL", databaseURL.String())
	api := newTestAPI(t)
	o := api.newOrder()
	v := proposeTestAdjustment(api, o, "worker", "increase", "adjust-single-connection", 25)
	respondTestAdjustment(api, v, "player", "accept", 200)
	o = payTestAdjustment(api, v, "wechat", "")
	api.Server.Client().Timeout = 2 * time.Second
	var result AnalyticsResult
	api.call("admin", "GET", "/admin/analytics", nil, 200, &result)
	if result.Summary.GMVCents != o.PaidTotalCents {
		t.Fatal("单连接池统计未完整核验补款")
	}
}
