// Package app 在隔离数据库和受控微信替身中验证虚拟金币资金守恒、未知扣币恢复、退币和现金退款来源冻结。
package app

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// virtualTestChannel 保存本地受控官方余额与稳定扣币记录，测试不连接真实支付账户。
type virtualTestChannel struct {
	mu           sync.Mutex
	Balance      int64
	Cash         map[string]bool
	Debits       map[string]int64
	Refunds      map[string]int64
	TimeoutDebit bool
	DebitCalls   int
}

// virtualFundsAPI 建立独立schema、Redis前缀、会话和固定官方API替身，普通商户配置也只使用临时测试密钥。
func virtualFundsAPI(t *testing.T, buyerFee bool) (*testAPI, *virtualTestChannel, wxPayConfig, *rsa.PrivateKey) {
	t.Helper()
	api := newTestAPI(t)
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	api.App.Config.DevMode = false
	api.App.Config.WechatAppID = c.AppID
	api.App.Config.WechatSecret = "virtual-isolated-secret"
	api.App.Config.WechatMessageToken = "virtual-test-callback-token"
	settings := commerceSettings{Mode: "mini_program", FeeBearer: "platform", AndroidFeeBPS: 100, IOSFeeBPS: 1200, OfferID: "12345", AppKey: "virtual-only-test-app-key"}
	if buyerFee {
		settings.FeeBearer = "buyer"
	}
	api.App.commerceConfig = &settings
	for _, user := range []string{"player", "worker", "outsider"} {
		identity, e := get[map[string]string](t.Context(), api.App.DB, "identities", user, false)
		if e != nil {
			t.Fatal(e)
		}
		identity["appid"] = c.AppID
		identity["openid"] = "virtual-open-" + user
		identity["phone_source"] = "wechat"
		if e = put(t.Context(), api.App.DB, "identities", user, identity); e != nil {
			t.Fatal(e)
		}
		if e = api.App.storeWechatSession(t.Context(), user, wechatSession{AppID: c.AppID, OpenID: identity["openid"], SessionKey: "test-session-" + user}); e != nil {
			t.Fatal(e)
		}
	}
	s, e := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	if e != nil {
		t.Fatal(e)
	}
	s.PriceCents = 10000
	if e = put(t.Context(), api.App.DB, "services", s.ID, s); e != nil {
		t.Fatal(e)
	}
	channel := &virtualTestChannel{Cash: map[string]bool{}, Debits: map[string]int64{}, Refunds: map[string]int64{}}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"access_token":"virtual-access","expires_in":7200}`))}, nil
		}
		channel.mu.Lock()
		defer channel.mu.Unlock()
		var input struct {
			OrderID    string `json:"order_id"`
			Amount     int64  `json:"amount"`
			PayOrderID string `json:"pay_order_id"`
		}
		if e := json.NewDecoder(r.Body).Decode(&input); e != nil {
			return nil, e
		}
		var body any
		switch r.URL.Path {
		case "/xpay/query_order":
			var id string
			if e := api.App.DB.QueryRow(t.Context(), "SELECT id FROM wallet_topups WHERE data->>'order_no'=$1", input.OrderID).Scan(&id); e != nil {
				return nil, e
			}
			p, e := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", id, false)
			if e != nil {
				return nil, e
			}
			if !channel.Cash[p.ID] {
				channel.Balance += p.AmountCents / 100
				channel.Cash[p.ID] = true
			}
			body = map[string]any{"errcode": 0, "order": virtualCashOrder{OrderID: p.OrderNo, Status: 4, OrderFee: p.AmountCents, PaidFee: p.AmountCents, OrderType: 0, WXOrderID: "WXV" + p.ID, PaidTime: time.Now().Unix(), LeftFee: p.AmountCents}}
		case "/xpay/query_user_balance":
			body = map[string]any{"errcode": 0, "balance": channel.Balance}
		case "/xpay/currency_pay":
			channel.DebitCalls++
			if old, ok := channel.Debits[input.OrderID]; ok {
				if old != input.Amount {
					t.Error("相同官方扣币号金额改变")
				}
				body = map[string]any{"errcode": 268490004}
			} else {
				if input.Amount <= 0 || channel.Balance < input.Amount {
					t.Error("官方金币超扣或按分误当金币")
				}
				channel.Debits[input.OrderID] = input.Amount
				channel.Balance -= input.Amount
				if channel.TimeoutDebit {
					channel.TimeoutDebit = false
					return nil, errors.New("模拟官方已扣币但网络超时")
				}
				body = map[string]any{"errcode": 0}
			}
		case "/xpay/cancel_currency_pay":
			if _, ok := channel.Debits[input.PayOrderID]; !ok {
				t.Error("退币未关联原扣币")
			}
			if old, ok := channel.Refunds[input.OrderID]; ok {
				if old != input.Amount {
					t.Error("相同退款号金额改变")
				}
				body = map[string]any{"errcode": 268490004}
			} else {
				channel.Refunds[input.OrderID] = input.Amount
				channel.Balance += input.Amount
				body = map[string]any{"errcode": 0}
			}
		default:
			return nil, errors.New("测试没有授权的微信接口")
		}
		raw, _ := json.Marshal(body)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}
	return api, channel, c, key
}

// beginVirtualOrder 通过正式HTTP入口创建服务单、确认服务端报价并取得差额购币参数。
func beginVirtualOrder(t *testing.T, api *testAPI) (Order, WalletTopup) {
	t.Helper()
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	var quote virtualQuote
	api.call("player", "POST", "/payments/virtual/quote", map[string]any{"order_id": order.ID, "platform": "android"}, 200, &quote)
	var result struct {
		Payment WalletTopup       `json:"payment"`
		Mode    string            `json:"mode"`
		Params  map[string]string `json:"virtual_params"`
	}
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{"platform": "android", "quoted_total_cents": quote.TotalCents, "quoted_topup_cents": quote.TopupCents}, 200, &result)
	if result.Mode != "virtual" || result.Params["mode"] != "short_series_coin" || result.Payment.Channel != "virtual" || strings.Contains(result.Params["signData"], "session_key") {
		t.Fatal("没有取得安全的原金币充值会话")
	}
	return order, result.Payment
}

// TestIntegrationVirtualUnknownDebitAndFeeRefund 验证未知扣币原单恢复、禁止取消在途支付、服务费隔离及全额退币守恒。
func TestIntegrationVirtualUnknownDebitAndFeeRefund(t *testing.T) {
	api, channel, _, _ := virtualFundsAPI(t, true)
	order, topup := beginVirtualOrder(t, api)
	channel.TimeoutDebit = true
	api.call("player", "POST", "/orders/"+order.ID+"/sync-payment", map[string]any{}, 502, nil)
	wallet := testWallet(api, "player")
	if wallet.VirtualCoinCents != 10100 || wallet.RechargeCents != 0 || wallet.AvailableCents != 0 {
		t.Fatalf("未知扣币错误修改钱包:%+v", wallet)
	}
	api.action("player", order.ID, "cancel", "取消在途付款", 409)
	for range 2 {
		api.call("player", "POST", "/orders/"+order.ID+"/sync-payment", map[string]any{}, 200, &order)
	}
	if order.PaymentStatus != "paid" || order.PaymentChannel != "virtual" || !order.HasVirtualPayment || order.AmountCents != 10000 || order.AdjustmentVersion != 1 || len(channel.Debits) != 1 || channel.DebitCalls != 2 {
		t.Fatalf("原单恢复丢失或重复资金:%+v", order)
	}
	wallet = testWallet(api, "player")
	if wallet.VirtualCoinCents != 0 || wallet.RechargeCents != 0 || wallet.DepositCents != 0 {
		t.Fatal("金币与人民币/押金混用")
	}
	worker := testWallet(api, "worker")
	if worker.PendingCents != order.Settlement.ProviderIncomeCents {
		t.Fatal("手续费被混入服务收入")
	}
	api.action("player", order.ID, "cancel", "退还本次服务", 200)
	wallet = testWallet(api, "player")
	if wallet.VirtualCoinCents != 10100 || wallet.RechargeCents != 0 || wallet.AvailableCents != 0 || len(channel.Refunds) != 1 || channel.Balance != 101 {
		t.Fatalf("全退未恢复服务金币和原服务费:%+v", wallet)
	}
	if e := api.App.recordVirtualCashRefund(t.Context(), topup, 1000, "cash-cumulative-1000", true); e != nil {
		t.Fatal(e)
	}
	if e := api.App.recordVirtualCashRefund(t.Context(), topup, 500, "late-cumulative-500", true); e != nil {
		t.Fatal(e)
	}
	if e := api.App.recordVirtualCashRefund(t.Context(), topup, 2000, "cash-cumulative-2000", true); e != nil {
		t.Fatal(e)
	}
	if e := api.App.recordVirtualCashRefund(t.Context(), topup, 2000, "cash-cumulative-2000", true); e != nil {
		t.Fatal(e)
	}
	stored, e := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", topup.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	if stored.RefundedCents != 2000 || testWallet(api, "player").VirtualCoinCents != 8100 {
		t.Fatal("迟到或重复现金退款倒退容量或重复扣币")
	}
}

// TestIntegrationVirtualIncomeFundingAndCashChargeback 验证只有履约收入可提现，官方充值现金退款后已流转来源必须冻结。
func TestIntegrationVirtualIncomeFundingAndCashChargeback(t *testing.T) {
	api, _, _, _ := virtualFundsAPI(t, false)
	order, topup := beginVirtualOrder(t, api)
	api.call("player", "POST", "/orders/"+order.ID+"/sync-payment", map[string]any{}, 200, &order)
	api.action("worker", order.ID, "accept", "", 200)
	api.action("worker", order.ID, "start", "", 200)
	api.action("worker", order.ID, "deliver", "已完成约定服务内容", 200)
	api.action("player", order.ID, "confirm", "", 200)
	if _, e := api.App.DB.Exec(t.Context(), "UPDATE orders SET data=jsonb_set(data,'{completed_at}',to_jsonb($2::text)) WHERE id=$1", order.ID, time.Now().Add(-8*24*time.Hour).Format(time.RFC3339Nano)); e != nil {
		t.Fatal(e)
	}
	wallet := testWallet(api, "worker")
	if wallet.AvailableCents != order.Settlement.ProviderIncomeCents {
		t.Fatal("已验收虚拟服务没有真实来源收入")
	}
	tx, e := api.App.DB.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(t.Context())
	if e = lockWalletUsers(t.Context(), tx, "worker"); e != nil {
		t.Fatal(e)
	}
	lots, e := fundingLots(t.Context(), tx, "worker", "available")
	if e != nil || len(lots) == 0 {
		t.Fatal("收入来源缺失")
	}
	for _, l := range lots {
		if e = verifyFundingLot(t.Context(), tx, l, false, map[string]bool{}); e != nil {
			t.Fatalf("合法虚拟收入不能核验:%v", e)
		}
	}
	if e = tx.Commit(t.Context()); e != nil {
		t.Fatal(e)
	}
	if e = api.App.recordVirtualCashRefund(t.Context(), topup, topup.AmountCents, "apple-cash-refund-after-service", false); e != nil {
		t.Fatal(e)
	}
	proof, e := get[virtualTransaction](t.Context(), api.App.DB, "virtual_transactions", topup.ID, false)
	if e != nil || !proof.ReconciliationRequired || proof.RecoveredCoinCents != 0 {
		t.Fatal("已消费来源退款未冻结")
	}
	for _, l := range lots {
		if e = verifyFundingLot(t.Context(), api.App.DB, l, false, map[string]bool{}); e == nil {
			t.Fatal("现金已退来源仍可提现")
		}
	}
	if testWallet(api, "player").VirtualCoinCents != 0 {
		t.Fatal("现金退款后向买家凭空补金币")
	}
}

// TestIntegrationVirtualSupplementPreservesOriginalChannel 验证普通微信旧单的虚拟补款、逐笔减价及最终混合退款不改写原收款。
func TestIntegrationVirtualSupplementPreservesOriginalChannel(t *testing.T) {
	api, channel, c, key := virtualFundsAPI(t, false)
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	stored, e := get[Order](t.Context(), api.App.DB, "orders", order.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	stored.PayOpenID = "virtual-open-player"
	stored.PaymentAppID = c.AppID
	stored.PaymentMerchantID = c.MerchantID
	if e = put(t.Context(), api.App.DB, "orders", stored.ID, stored); e != nil {
		t.Fatal(e)
	}
	v := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: stored.OrderNo, TransactionID: "NORMALBASE" + stored.ID, TradeState: "SUCCESS", SuccessTime: now()}
	v.Payer.OpenID = stored.PayOpenID
	v.Amount.Total = stored.AmountCents
	v.Amount.Currency = "CNY"
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", v, ""); code != 204 {
		t.Fatalf("普通微信旧单失败:%d", code)
	}
	api.call("player", "GET", "/orders/"+order.ID, nil, 200, &order)
	proposal := proposeTestAdjustment(api, order, "worker", "increase", "virtual-mixed-supplement", 1000)
	proposal = respondTestAdjustment(api, proposal, "player", "accept", 200)
	var quote virtualQuote
	api.call("player", "POST", "/payments/virtual/quote", map[string]any{"order_id": order.ID, "adjustment_id": proposal.ID, "platform": "android"}, 200, &quote)
	path := "/orders/" + order.ID + "/adjustments/" + proposal.ID
	api.call("player", "POST", path+"/pay", map[string]any{"platform": "android", "quoted_total_cents": quote.TotalCents, "quoted_topup_cents": quote.TopupCents}, 200, nil)
	api.call("player", "POST", path+"/sync", map[string]any{}, 200, &proposal)
	api.call("player", "GET", "/orders/"+order.ID, nil, 200, &order)
	if proposal.PaymentChannel != "virtual" || !order.HasVirtualPayment || order.PaymentChannel != "wechat" || order.AmountCents != 11000 || order.OriginalAmountCents != 10000 || len(channel.Debits) != 1 {
		t.Fatalf("虚拟补款改写原普通支付:%+v %+v", order, proposal)
	}
	decrease := proposeTestAdjustment(api, order, "player", "decrease", "virtual-mixed-decrease", 500)
	respondTestAdjustment(api, decrease, "worker", "accept", 200)
	if testWallet(api, "player").VirtualCoinCents != 500 || len(channel.Refunds) != 1 {
		t.Fatal("减价没有按最新虚拟原单退币")
	}
	transport := api.App.HTTPClient.Transport
	api.App.HTTPClient.Transport = paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v3/refund/domestic/refunds" {
			return transport.RoundTrip(r)
		}
		var in struct {
			TransactionID string `json:"transaction_id"`
			OutRefundNo   string `json:"out_refund_no"`
			Amount        struct {
				Total  int64 `json:"total"`
				Refund int64 `json:"refund"`
			} `json:"amount"`
		}
		if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
			return nil, e
		}
		if in.TransactionID != v.TransactionID || in.Amount.Total != 10000 || in.Amount.Refund != 10000 {
			t.Error("普通微信退款未沿用原交易")
		}
		result := wxRefund{MerchantID: c.MerchantID, OrderNo: stored.OrderNo, TransactionID: v.TransactionID, OutRefundNo: in.OutRefundNo, Status: "SUCCESS", SuccessTime: now()}
		result.Amount.Total = in.Amount.Total
		result.Amount.Refund = in.Amount.Refund
		result.Amount.Currency = "CNY"
		return signedAdjustmentResponse(t, c, key, result), nil
	})
	api.action("player", order.ID, "cancel", "混合渠道全退", 200)
	if testWallet(api, "player").VirtualCoinCents != 1000 || channel.Balance != 10 || len(channel.Refunds) != 2 || testWallet(api, "worker").PendingCents != 0 {
		t.Fatal("混合渠道全退未恢复原金币或清理待结算收入")
	}
	final, e := get[Order](t.Context(), api.App.DB, "orders", order.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	if final.PaymentTransactionID != v.TransactionID || final.PaymentAppID != c.AppID || final.PaymentChannel != "wechat" || final.Status != "refunded" {
		t.Fatal("混合退款改写初付款凭证或未关闭")
	}
}
