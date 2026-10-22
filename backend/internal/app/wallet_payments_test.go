// Package app 用一次性微信测试密钥验证真实通知协议、充值重放、押金分笔退款补偿和提现未知结果冻结，不调用真实商户。
package app

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// productionWalletAPI 切换到真实协议校验但以临时密钥和本地隔离schema运行，不接触线上商户或真实用户资金。
func productionWalletAPI(t *testing.T) (*testAPI, wxPayConfig, *rsa.PrivateKey) {
	t.Helper()
	api := newTestAPI(t)
	c, key := testPayConfig(t)
	configurePayEnvironment(t, c)
	api.App.Config.DevMode = false
	api.App.Config.WechatAppID = c.AppID
	for _, id := range []string{"player", "worker", "outsider", "admin"} {
		identity, err := get[map[string]string](t.Context(), api.App.DB, "identities", id, false)
		if err != nil {
			t.Fatal(err)
		}
		identity["openid"] = "wallet-openid-" + id
		identity["phone_source"] = "wechat"
		if err = put(t.Context(), api.App.DB, "identities", id, identity); err != nil {
			t.Fatal(err)
		}
	}
	return api, c, key
}

// deliverWalletNotice 把真实AES-GCM密文和RSA签名送入相同的官方通知处理器。
func deliverWalletNotice(t *testing.T, api *testAPI, c wxPayConfig, key *rsa.PrivateKey, event string, payload any, kind string) int {
	t.Helper()
	raw := encryptedNotice(t, c, event, payload)
	request := httptest.NewRequest("POST", "/notify", bytes.NewReader(raw))
	request.Header = signedHeader(t, c, key, raw)
	recorder := httptest.NewRecorder()
	switch kind {
	case "refund":
		api.App.refundCallback(recorder, request)
	case "transfer":
		api.App.transferCallback(recorder, request)
	default:
		api.App.paymentCallback(recorder, request)
	}
	return recorder.Code
}

// pendingWXTopup 建立带原支付身份的未付款测试意图，只有后续签名通知才会增加余额。
func pendingWXTopup(t *testing.T, api *testAPI, user, purpose string, amount int64) WalletTopup {
	t.Helper()
	id := newID()
	p := WalletTopup{ID: id, UserID: user, RequestID: "protocol-" + id, Purpose: purpose, AmountCents: amount, Status: "pending_payment", OrderNo: "WT" + strings.ToUpper(id[:30]), PayOpenID: "wallet-openid-" + user, CreatedAt: now(), UpdatedAt: now()}
	if err := put(t.Context(), api.App.DB, "wallet_topups", id, p); err != nil {
		t.Fatal(err)
	}
	return p
}

// wxTopupTransaction 构造与原充值绑定的官方成功资源，测试会分别篡改关键字段。
func wxTopupTransaction(c wxPayConfig, p WalletTopup) wxTransaction {
	v := wxTransaction{AppID: c.AppID, MerchantID: c.MerchantID, OrderNo: p.OrderNo, TransactionID: "WX" + p.ID, TradeState: "SUCCESS"}
	v.Payer.OpenID = p.PayOpenID
	v.Amount.Total = p.AmountCents
	v.Amount.Currency = "CNY"
	return v
}

// TestIntegrationWalletWXTopupNotifications 验证金额、商户和收款身份核对、重复回放以及原单恢复支付。
func TestIntegrationWalletWXTopupNotifications(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	p := pendingWXTopup(t, api, "player", "balance", 1000)
	paid := wxTopupTransaction(c, p)
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		raw := []byte(`{"prepay_id":"wallet-test-prepay"}`)
		return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, raw), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}
	api.call("outsider", "POST", "/wallet/topups/"+p.ID+"/pay", map[string]any{}, 403, nil)
	api.call("player", "POST", "/wallet/topups/"+p.ID+"/pay", map[string]any{}, 200, nil)
	if wallet := testWallet(api, "player"); wallet.RechargeCents != 0 {
		t.Fatal("预支付会话被当成充值成功")
	}
	for _, mutate := range []func(*wxTransaction){func(v *wxTransaction) { v.Amount.Total++ }, func(v *wxTransaction) { v.MerchantID = "wrong" }, func(v *wxTransaction) { v.Payer.OpenID = "wrong" }} {
		bad := paid
		mutate(&bad)
		if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", bad, ""); code != 400 {
			t.Fatalf("篡改充值通知未拒绝:%d", code)
		}
	}
	for range 3 {
		if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", paid, ""); code != 204 {
			t.Fatalf("正确充值通知失败:%d", code)
		}
	}
	if wallet := testWallet(api, "player"); wallet.RechargeCents != 1000 || wallet.AvailableCents != 0 {
		t.Fatalf("充值重放多入账:%+v", wallet)
	}
	api.call("player", "POST", "/wallet/topups/"+p.ID+"/pay", map[string]any{}, 200, nil)
	conflict := paid
	conflict.TransactionID = "OTHER"
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", conflict, ""); code != 409 {
		t.Fatalf("重复单不同流水未拒绝:%d", code)
	}
}

// TestIntegrationWalletWXDepositRefundTimeout 验证多笔部分退款出网超时仍冻结、原单补偿、金额校验和已退款充值迟到通知不会重复入账。
func TestIntegrationWalletWXDepositRefundTimeout(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	sources := map[string]WalletTopup{}
	for _, amount := range []int64{200, 300} {
		p := pendingWXTopup(t, api, "worker", "deposit", amount)
		sources["WX"+p.ID] = p
		if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", wxTopupTransaction(c, p), ""); code != 204 {
			t.Fatal(code)
		}
	}
	var mu sync.Mutex
	timeout := true
	refunds := map[string]wxRefund{}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		var result wxRefund
		if r.Method == "POST" {
			var payload struct {
				TransactionID string `json:"transaction_id"`
				RefundNo      string `json:"out_refund_no"`
				Amount        struct {
					Refund   int64  `json:"refund"`
					Total    int64  `json:"total"`
					Currency string `json:"currency"`
				} `json:"amount"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			p, ok := sources[payload.TransactionID]
			if !ok || payload.Amount.Total != p.AmountCents || payload.Amount.Refund > p.AmountCents {
				t.Error("退款未按原支付来源和部分金额提交")
			}
			result = wxRefund{MerchantID: c.MerchantID, OrderNo: p.OrderNo, OutRefundNo: payload.RefundNo, TransactionID: payload.TransactionID, Status: "SUCCESS"}
			result.Amount.Total = p.AmountCents
			result.Amount.Refund = payload.Amount.Refund
			result.Amount.Currency = "CNY"
			refunds[result.OutRefundNo] = result
			if timeout {
				timeout = false
				return nil, errors.New("模拟微信已受理但客户端连接超时")
			}
		} else {
			result = refunds[r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]]
		}
		raw, _ := json.Marshal(result)
		return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, raw), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}
	api.call("worker", "POST", "/wallet/deposit-refunds", map[string]any{"amount_cents": 400, "request_id": "wx-deposit-refund-0001"}, 502, nil)
	var requests []WalletDepositRefund
	api.call("worker", "GET", "/wallet/deposit-refunds", nil, 200, &requests)
	if len(requests) != 1 {
		t.Fatal(requests)
	}
	refund := requests[0]
	wallet := testWallet(api, "worker")
	if wallet.DepositCents != 100 || wallet.DepositRefundingCents != 400 {
		t.Fatalf("超时错误解冻:%+v", wallet)
	}
	var service Service
	api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
	if service.ProviderDepositCents != 100 {
		t.Fatalf("退还中押金仍计有效认证:%+v", service)
	}
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 409, nil)
	api.call("outsider", "POST", "/wallet/deposit-refunds/"+refund.ID+"/sync", map[string]any{}, 403, nil)
	api.call("worker", "POST", "/wallet/deposit-refunds/"+refund.ID+"/sync", map[string]any{}, 200, &refund)
	if refund.Status != "success" || len(refund.Parts) != 2 {
		t.Fatalf("原单补偿未完成多笔:%+v", refund)
	}
	wallet = testWallet(api, "worker")
	if wallet.DepositCents != 100 || wallet.DepositRefundingCents != 0 {
		t.Fatalf("成功退款金额错误:%+v", wallet)
	}
	for _, p := range sources {
		if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", wxTopupTransaction(c, p), ""); code != 204 {
			t.Fatal(code)
		}
	}
	if wallet = testWallet(api, "worker"); wallet.DepositCents != 100 {
		t.Fatal("迟到付款重建已退押金")
	}
	for _, result := range refunds {
		bad := result
		bad.Amount.Refund++
		if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", func() wxRefund { bad.RefundStatus = "SUCCESS"; return bad }(), "refund"); code != 400 {
			t.Fatalf("错退款金额被接受:%d", code)
		}
		result.RefundStatus = "SUCCESS"
		if code := deliverWalletNotice(t, api, c, key, "REFUND.SUCCESS", result, "refund"); code != 204 {
			t.Fatal(code)
		}
	}
}

// TestIntegrationWalletWXWithdrawalUnknown 验证转账超时冻结、可选openid原单查询、本人确认参数、通知金额商户核对和明确失败解冻。
func TestIntegrationWalletWXWithdrawalUnknown(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	testIncome(api, "worker", 1000)
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 400, "request_id": "wx-withdraw-0001"}, 503, nil)
	if wallet := testWallet(api, "worker"); wallet.WithdrawalFrozenCents != 0 || wallet.AvailableCents != 1000 {
		t.Fatal("缺少转账配置仍冻结")
	}
	settings := WalletSettings{WithdrawalEnabled: true, TransferSceneID: "test-approved-scene", TransferNotifyURL: "https://example.com/api/payments/wechat/transfer-notify", TransferRemark: "服务收入提现", TransferSceneReportInfos: []WalletSceneReport{{InfoType: "岗位类型", InfoContent: "测试服务收入"}}}
	api.call("admin", "PATCH", "/admin/wallet-settings", settings, 200, nil)
	var withdrawal WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 400, "request_id": "wx-withdraw-0001"}, 200, &withdrawal)
	var mu sync.Mutex
	timeout := true
	records := map[string]wxTransfer{}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		var result wxTransfer
		if r.Method == "POST" {
			var payload struct {
				OutBillNo string `json:"out_bill_no"`
				OpenID    string `json:"openid"`
				Amount    int64  `json:"transfer_amount"`
				Scene     string `json:"transfer_scene_id"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.OpenID != "wallet-openid-worker" || payload.Scene != settings.TransferSceneID {
				t.Error("提现未绑定原收款人和获批场景")
			}
			result = wxTransfer{OutBillNo: payload.OutBillNo, TransferBillNo: "WXTRANSFER" + payload.OutBillNo, MerchantID: c.MerchantID, AppID: c.AppID, AmountCents: payload.Amount, State: "WAIT_USER_CONFIRM", PackageInfo: "test-confirmation-package"}
			records[payload.OutBillNo] = result
			if timeout {
				timeout = false
				return nil, errors.New("模拟微信已受理后超时")
			}
		} else {
			result = records[r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]]
		}
		raw, _ := json.Marshal(result)
		return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, raw), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+withdrawal.ID+"/execute", map[string]any{}, 502, nil)
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 600 || wallet.WithdrawalFrozenCents != 400 {
		t.Fatalf("转账超时错误解冻:%+v", wallet)
	}
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+withdrawal.ID+"/reject", map[string]string{"reason": "未知时不能解冻"}, 409, nil)
	var synced map[string]any
	api.call("worker", "POST", "/wallet/withdrawals/"+withdrawal.ID+"/sync", map[string]any{}, 200, &synced)
	if synced["status"] != "wait_user_confirm" || synced["confirmation"] == nil {
		t.Fatalf("查询省略openid时未返回本人确认参数:%+v", synced)
	}
	for _, field := range []string{"package_info", "pay_open_id", "encrypted_name", "name_hash"} {
		if _, ok := synced[field]; ok {
			t.Fatalf("私密字段泄露:%s", field)
		}
	}
	var adminList []map[string]any
	api.call("admin", "GET", "/admin/wallet/withdrawals", nil, 200, &adminList)
	if len(adminList) != 1 || adminList[0]["confirmation"] != nil {
		t.Fatal("管理员列表泄露本人收款确认包")
	}
	mu.Lock()
	result := records[withdrawal.OutBillNo]
	mu.Unlock()
	result.OpenID = "wallet-openid-worker"
	result.State = "SUCCESS"
	for _, mutate := range []func(*wxTransfer){func(v *wxTransfer) { v.AmountCents++ }, func(v *wxTransfer) { v.MerchantID = "wrong" }, func(v *wxTransfer) { v.OpenID = "wrong" }} {
		bad := result
		mutate(&bad)
		if code := deliverWalletNotice(t, api, c, key, "MCHTRANSFER.BILL.FINISHED", bad, "transfer"); code != 400 {
			t.Fatalf("篡改提现通知未拒绝:%d", code)
		}
	}
	for range 2 {
		if code := deliverWalletNotice(t, api, c, key, "MCHTRANSFER.BILL.FINISHED", result, "transfer"); code != 204 {
			t.Fatalf("提现成功通知失败:%d", code)
		}
	}
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 600 || wallet.WithdrawalFrozenCents != 0 {
		t.Fatalf("成功提现重复扣除:%+v", wallet)
	}
	badTerminal := result
	badTerminal.State = "FAIL"
	if code := deliverWalletNotice(t, api, c, key, "MCHTRANSFER.BILL.FINISHED", badTerminal, "transfer"); code != 409 {
		t.Fatalf("冲突终态覆盖成功:%d", code)
	}
	var failed WalletWithdrawal
	api.call("worker", "POST", "/wallet/withdrawals", map[string]any{"amount_cents": 300, "request_id": "wx-withdraw-0002"}, 200, &failed)
	api.call("admin", "POST", "/admin/wallet/withdrawals/"+failed.ID+"/execute", map[string]any{}, 200, nil)
	mu.Lock()
	failure := records[failed.OutBillNo]
	mu.Unlock()
	failure.OpenID = "wallet-openid-worker"
	failure.State = "FAIL"
	failure.FailReason = "模拟官方明确失败"
	for range 2 {
		if code := deliverWalletNotice(t, api, c, key, "MCHTRANSFER.BILL.FINISHED", failure, "transfer"); code != 204 {
			t.Fatal(code)
		}
	}
	if wallet := testWallet(api, "worker"); wallet.AvailableCents != 600 || wallet.WithdrawalFrozenCents != 0 {
		t.Fatalf("明确失败未只解冻一次:%+v", wallet)
	}
}

// TestIntegrationWalletProductionBalanceRefund 验证真实协议环境下的余额退款走原余额分账，不误调用微信原支付退款。
func TestIntegrationWalletProductionBalanceRefund(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	p := pendingWXTopup(t, api, "player", "balance", 3000)
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", wxTopupTransaction(c, p), ""); code != 204 {
		t.Fatal(code)
	}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		t.Error("余额退款不应调用微信")
		return nil, errors.New("禁止外部请求")
	})}
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{"channel": "balance", "request_id": "real-balance-order-0001"}, 200, nil)
	api.action("player", order.ID, "cancel", "", 200)
	if wallet := testWallet(api, "player"); wallet.RechargeCents != 3000 || wallet.AvailableCents != 0 {
		t.Fatalf("真实模式余额退款未还原:%+v", wallet)
	}
	if wallet := testWallet(api, "worker"); wallet.PendingCents != 0 {
		t.Fatal("余额退款未撤回托管")
	}
}

// TestIntegrationWalletPaymentChannelIntent 验证已发起微信预支付的订单不能同时扣余额，避免迟到微信付款导致双重支出。
func TestIntegrationWalletPaymentChannelIntent(t *testing.T) {
	api, c, key := productionWalletAPI(t)
	p := pendingWXTopup(t, api, "player", "balance", 3000)
	if code := deliverWalletNotice(t, api, c, key, "TRANSACTION.SUCCESS", wxTopupTransaction(c, p), ""); code != 204 {
		t.Fatal(code)
	}
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		raw := []byte(`{"prepay_id":"test-order-payment"}`)
		return &http.Response{StatusCode: 200, Header: signedHeader(t, c, key, raw), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]string{"channel": "wechat"}, 200, nil)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]string{"channel": "balance", "request_id": "reject-channel-switch-0001"}, 409, nil)
	if wallet := testWallet(api, "player"); wallet.RechargeCents != 3000 {
		t.Fatal("微信预付意图存在时仍扣了余额")
	}
}
