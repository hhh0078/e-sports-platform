// Package app 验证管理统计的身份隔离、上海自然日边界、历史数据限制和真实付款/退款时间回放幂等。
package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestAnalyticsDateRange 验证默认上海近七日、闰年366日闭区间及全部非法日期边界。
func TestAnalyticsDateRange(t *testing.T) {
	stamp := time.Date(2026, 10, 20, 16, 0, 0, 0, time.UTC)
	period, days, _, err := analyticsDates(httptest.NewRequest("GET", "/analytics", nil), stamp)
	if err != nil || period.Start != "2026-10-15" || period.End != "2026-10-21" || period.Timezone != "Asia/Shanghai" || len(days) != 7 {
		t.Fatalf("默认周期未按上海日期:%+v %d %v", period, len(days), err)
	}
	_, days, _, err = analyticsDates(httptest.NewRequest("GET", "/analytics?start=2024-01-01&end=2024-12-31", nil), stamp)
	if err != nil || len(days) != 366 {
		t.Fatal("闰年366日不应被拒绝", err)
	}
	for _, query := range []string{"start=2026-10-21", "end=2026-10-21", "start=&end=", "start=2026-2-01&end=2026-02-02", "start=2026-02-30&end=2026-03-01", "start=2026-10-22&end=2026-10-21", "start=2024-01-01&end=2025-01-01", "start=abc&end=2026-10-21"} {
		t.Run(query, func(t *testing.T) {
			if _, _, _, err := analyticsDates(httptest.NewRequest("GET", "/analytics?"+query, nil), stamp); err == nil {
				t.Fatal("无效统计周期被接受")
			}
		})
	}
}

// TestIntegrationAnalyticsAdminAndEmpty 统计仅限平台管理员，空周期完整补零且响应不含个人身份或商户配置。
func TestIntegrationAnalyticsAdminAndEmpty(t *testing.T) {
	api := newTestAPI(t)
	for _, actor := range []string{"", "player", "worker", "clubowner", "outsider"} {
		code := 403
		if actor == "" {
			code = 401
		}
		api.call(actor, "GET", "/admin/analytics?start=2026-10-20&end=2026-10-22", nil, code, nil)
	}
	var out AnalyticsResult
	api.call("admin", "GET", "/admin/analytics?start=2026-10-20&end=2026-10-22", nil, 200, &out)
	if len(out.Series) != 3 || out.Series[0].Date != "2026-10-20" || out.Series[2].Date != "2026-10-22" || out.Summary.GMVCents != 0 || out.Summary.RefundCents != 0 || !out.DevMode || out.DataMode != "local" || out.StatisticsNote == "" {
		t.Fatalf("空周期或本地标识不正确:%+v", out)
	}
	raw, _ := json.Marshal(out)
	for _, private := range []string{"openid", "merchant_id", "api_v3_key", "buyer_id", "provider_id", "certificate_serial", "private_key"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("统计返回私密字段", private)
		}
	}
	api.call("admin", "GET", "/admin/analytics?start=2026-02-30&end=2026-03-01", nil, 400, nil)
}

// analyticsFixture 仅在隔离测试库构造已核账服务订单及其逐单资金来源，用于精确检验跨日金额。
func analyticsFixture(t *testing.T, api *testAPI, id, channel, paid, refunded string, amount int64) Order {
	t.Helper()
	o := Order{ID: id, OrderNo: "AN" + id, BuyerID: "player", ProviderID: "worker", AmountCents: amount, WalletAccounted: true, PaymentChannel: channel, PaidAt: paid, RefundedAt: refunded, PaymentStatus: "paid", Status: "open", PaymentTransactionID: "WX-" + id, CreatedAt: "2000-01-01T00:00:00Z", UpdatedAt: "2099-01-01T00:00:00Z"}
	e := WalletEscrow{OrderID: id, BuyerID: o.BuyerID, ProviderID: o.ProviderID, AmountCents: amount, Channel: channel, Status: "holding", CreatedAt: now()}
	if channel == "balance" {
		e.RechargeUsed = amount
	}
	if refunded != "" {
		o.PaymentStatus = "refunded"
		o.Status = "refunded"
		o.RefundID = "RF" + id
		e.Status = "refunded"
	}
	if err := put(t.Context(), api.App.DB, "orders", id, o); err != nil {
		t.Fatal(err)
	}
	if err := put(t.Context(), api.App.DB, "wallet_escrows", id, e); err != nil {
		t.Fatal(err)
	}
	return o
}

// TestIntegrationAnalyticsShanghaiAndCrossPeriod 验证UTC跨上海午夜、精确边界、退款跨期、假付款状态及旧缺时间记录不冒充交易额。
func TestIntegrationAnalyticsShanghaiAndCrossPeriod(t *testing.T) {
	api := newTestAPI(t)
	analyticsFixture(t, api, "before-midnight", "wechat", "2026-10-20T15:59:59.999999999Z", "", 100)
	analyticsFixture(t, api, "at-midnight", "wechat", "2026-10-20T16:00:00Z", "2026-10-22T16:00:00Z", 200)
	analyticsFixture(t, api, "end-last-nano", "balance", "2026-10-22T15:59:59.999999999Z", "", 300)
	analyticsFixture(t, api, "outside-period", "wechat", "2026-10-22T16:00:00Z", "", 400)
	analyticsFixture(t, api, "explicit-offset", "wechat", "2026-10-21T12:00:00+08:00", "", 500)
	analyticsFixture(t, api, "local-only", "dev", "2026-10-21T12:00:00+08:00", "", 50)
	analyticsFixture(t, api, "missing-paid-time", "wechat", "", "", 900)
	analyticsFixture(t, api, "bad-paid-time", "wechat", "not-a-time", "", 900)
	analyticsFixture(t, api, "demo_analytics", "wechat", "2026-10-21T12:00:00+08:00", "", 99999)
	analyticsFixture(t, api, "flagged-demo", "wechat", "2026-10-21T12:00:00+08:00", "", 99999)
	if _, err := api.App.DB.Exec(t.Context(), `UPDATE orders SET data=data||'{"is_demo":true}'::jsonb WHERE id='flagged-demo'`); err != nil {
		t.Fatal(err)
	}
	fake := analyticsFixture(t, api, "fake-without-source", "wechat", "2026-10-21T12:00:00+08:00", "", 99999)
	fake.WalletAccounted = false
	put(t.Context(), api.App.DB, "orders", fake.ID, fake)
	devTxn := analyticsFixture(t, api, "fake-dev-wechat", "wechat", "2026-10-21T12:00:00+08:00", "", 99999)
	devTxn.PaymentTransactionID = "DEV-fake"
	put(t.Context(), api.App.DB, "orders", devTxn.ID, devTxn)
	var out AnalyticsResult
	api.call("admin", "GET", "/admin/analytics?start=2026-10-20&end=2026-10-22", nil, 200, &out)
	if out.Summary.GMVCents != 1150 || out.Summary.PaidOrderCount != 5 || out.Summary.RefundCents != 0 || out.LegacyUntrackedOrders != 4 {
		t.Fatalf("交易口径错误:%+v", out)
	}
	if out.Series[0].PaidCents != 100 || out.Series[1].PaidCents != 750 || out.Series[2].PaidCents != 300 {
		t.Fatalf("上海跨日分桶错误:%+v", out.Series)
	}
	api.call("admin", "GET", "/admin/analytics?start=2026-10-23&end=2026-10-23", nil, 200, &out)
	if len(out.Series) != 1 || out.Summary.GMVCents != 400 || out.Summary.RefundCents != 200 || out.Summary.RefundedOrderCount != 1 || out.Series[0].RefundedOrders != 1 {
		t.Fatalf("跨期退款倒扣原付款日:%+v", out)
	}
	api.App.Config.DevMode = false
	api.call("admin", "GET", "/admin/analytics?start=2026-10-20&end=2026-10-22", nil, 200, &out)
	if out.Summary.GMVCents != 1100 || out.Summary.PaidOrderCount != 4 || out.LegacyUntrackedOrders != 5 || out.DataMode != "production" || out.DevMode {
		t.Fatalf("真实模式混入开发托管:%+v", out)
	}
}

// TestIntegrationAnalyticsPaymentHooksAndWalletExclusion 验证付款与退款首次记录时间，重复付款不重算，钱包充值押金均不进入服务成交额。
func TestIntegrationAnalyticsPaymentHooksAndWalletExclusion(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	paid, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = time.Parse(time.RFC3339Nano, paid.PaidAt); err != nil {
		t.Fatal("新实付款未记录可靠时间")
	}
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 200, nil)
	replay, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if replay.PaidAt != paid.PaidAt {
		t.Fatal("重复付款改变原实付时间")
	}
	testTopup(api, "player", "balance", "analytics-recharge-0001", 10000)
	testTopup(api, "player", "deposit", "analytics-deposit-0001", 1000)
	location, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Now().In(location).Format("2006-01-02")
	route := "/admin/analytics?start=" + today + "&end=" + today
	var out AnalyticsResult
	api.call("admin", "GET", route, nil, 200, &out)
	if out.Summary.GMVCents != 3800 || out.Summary.PaidOrderCount != 1 {
		t.Fatalf("充值押金计入GMV或重复付款多计:%+v", out)
	}
	refunded := api.action("player", o.ID, "cancel", "", 200)
	if refunded.RefundedAt == "" {
		t.Fatal("退款终态未记录发生时间")
	}
	api.call("admin", "GET", route, nil, 200, &out)
	if out.Summary.GMVCents != 3800 || out.Summary.RefundCents != 3800 || out.Summary.RefundedOrderCount != 1 {
		t.Fatalf("原始实付或退款口径错误:%+v", out)
	}
}

// TestIntegrationAnalyticsWXOfficialTimeAndReplay 验证官方签名付款和退款时间优先于回调接收时间，重复通知不改变日期或金额。
func TestIntegrationAnalyticsWXOfficialTimeAndReplay(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	o := baseOrder(User{ID: "player", Nickname: "隐私名称不可输出"})
	o.ProviderID = "worker"
	o.AmountCents = 1900
	o.PayOpenID = "wallet-openid-player"
	if err := put(t.Context(), api.App.DB, "orders", o.ID, o); err != nil {
		t.Fatal(err)
	}
	paid := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: o.OrderNo, TransactionID: "WX-analytics-transaction", TradeState: "SUCCESS", SuccessTime: "2026-10-21T00:00:00+08:00"}
	paid.Payer.OpenID = o.PayOpenID
	paid.Amount.Total = o.AmountCents
	paid.Amount.Currency = "CNY"
	for range 2 {
		if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", paid, ""); code != 204 {
			t.Fatal(code)
		}
	}
	stored, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if stored.PaidAt != "2026-10-20T16:00:00Z" {
		t.Fatalf("未使用官方实付时间:%s", stored.PaidAt)
	}
	stored.RefundID = "RF" + o.ID
	stored.RefundStatus = "processing"
	put(t.Context(), api.App.DB, "orders", o.ID, stored)
	refund := wxRefund{MerchantID: c.MerchantID, OrderNo: o.OrderNo, OutRefundNo: stored.RefundID, TransactionID: paid.TransactionID, RefundStatus: "SUCCESS", SuccessTime: "2026-10-23T00:00:00+08:00"}
	refund.Amount.Total = o.AmountCents
	refund.Amount.Refund = o.AmountCents
	refund.Amount.Currency = "CNY"
	for range 2 {
		if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", refund, "refund"); code != 204 {
			t.Fatal(code)
		}
	}
	var out AnalyticsResult
	api.call("admin", "GET", "/admin/analytics?start=2026-10-21&end=2026-10-21", nil, 200, &out)
	if out.Summary.GMVCents != 1900 || out.Summary.PaidOrderCount != 1 || out.Summary.RefundCents != 0 {
		t.Fatalf("实付日期或重放错误:%+v", out)
	}
	api.call("admin", "GET", "/admin/analytics?start=2026-10-23&end=2026-10-23", nil, 200, &out)
	if out.Summary.GMVCents != 0 || out.Summary.RefundCents != 1900 || out.Summary.RefundedOrderCount != 1 {
		t.Fatalf("官方跨日退款错误:%+v", out)
	}
}
