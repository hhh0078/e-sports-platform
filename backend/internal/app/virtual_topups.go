// Package app 将官方代币现金充值与人民币钱包隔离，按原单查证后才生成金币余额和不可变资金来源。
package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// newVirtualTopup 只创建整金币充值意图，不在本地预先增加余额或生成虚构的现金凭据。
func (a *App) newVirtualTopup(ctx context.Context, q querier, u User, id, requestID string, amount int64) (WalletTopup, error) {
	var p WalletTopup
	if a.Config.WechatMessageToken == "" {
		return p, fail(503, "虚拟支付消息验签尚未配置，请先接通官方退款通知")
	}
	if amount < 100 || amount > 20000000 || amount%100 != 0 {
		return p, fail(409, "微信1:1金币目前仅支持整金币充值，金额未被四舍五入")
	}
	c := a.commerceSettings()
	if _, _, e := a.virtualConfig(a.Config.WechatAppID, c.OfferID, c.VirtualEnv); e != nil {
		return p, e
	}
	s, e := a.miniProgramSessionWith(ctx, q, u.ID)
	if e != nil {
		return p, e
	}
	if _, _, e = a.virtualConfig(s.AppID, c.OfferID, c.VirtualEnv); e != nil {
		return p, e
	}
	p = WalletTopup{ID: id, AccountID: u.ID, UserID: u.ID, RequestID: requestID, Purpose: "balance", AmountCents: amount, Channel: "virtual", OfferID: c.OfferID, VirtualEnv: c.VirtualEnv, MerchantID: "virtual:" + c.OfferID, AppID: s.AppID, PayOpenID: s.OpenID, Status: "pending_payment", OrderNo: "VC" + strings.ToUpper(id[:30]), CreatedAt: now(), UpdatedAt: now()}
	return p, put(ctx, q, "wallet_topups", p.ID, p)
}

// virtualTopupPayment 生成与原充值完全一致的客户端签名，刷新会话只重签而不更换现金订单号。
func (a *App) virtualTopupPayment(r *http.Request, u User, p WalletTopup) (any, error) {
	if p.Status != "pending_payment" {
		return map[string]any{"payment": p, "mode": "virtual"}, nil
	}
	if e := requireAdultIdentity(r.Context(), a.DB, u.ID); e != nil {
		return nil, e
	}
	s, e := a.miniProgramSession(r.Context(), u.ID)
	if e != nil {
		return nil, e
	}
	if p.UserID != u.ID || p.PayOpenID != s.OpenID || p.AppID != s.AppID || p.Purpose != "balance" || p.Channel != "virtual" {
		return nil, fail(409, "原金币充值身份不一致")
	}
	_, key, e := a.virtualConfig(p.AppID, p.OfferID, p.VirtualEnv)
	if e != nil {
		return nil, e
	}
	if p.AmountCents <= 0 || p.AmountCents%100 != 0 {
		return nil, fail(409, "原充值金额不是整金币，禁止改变金额重签")
	}
	raw, e := json.Marshal(struct {
		OfferID      string `json:"offerId"`
		BuyQuantity  int64  `json:"buyQuantity"`
		Env          int    `json:"env"`
		CurrencyType string `json:"currencyType"`
		OutTradeNo   string `json:"outTradeNo"`
		Attach       string `json:"attach"`
	}{p.OfferID, p.AmountCents / 100, p.VirtualEnv, "CNY", p.OrderNo, p.ID})
	if e != nil {
		return nil, e
	}
	return map[string]any{"payment": p, "mode": "virtual", "virtual_params": map[string]string{"mode": "short_series_coin", "signData": string(raw), "paySig": virtualHMAC(key, "requestVirtualPayment&"+string(raw)), "signature": virtualHMAC(s.SessionKey, string(raw))}}, nil
}

// validateVirtualCashIdentity 检查不可变原单、面额和现金订单类型；不把上游处理状态误报为金额冲突。
func validateVirtualCashIdentity(p WalletTopup, o virtualCashOrder) error {
	if p.Channel != "virtual" || p.Purpose != "balance" || o.OrderID != p.OrderNo {
		return fail(409, "官方金币充值原单不一致，请平台核账")
	}
	if o.OrderFee != p.AmountCents || o.PaidFee != p.AmountCents {
		return fail(409, "官方金币充值面额或实付金额不一致，请平台核账")
	}
	if o.OrderType != 0 && o.OrderType != 7 {
		return fail(409, "官方返回的订单不是金币现金充值单，请平台核账")
	}
	return nil
}

// validateVirtualCashOrder 仅接受已付终态及完整可信支付凭据作为金币入账证据。
func validateVirtualCashOrder(p WalletTopup, o virtualCashOrder) error {
	if e := validateVirtualCashIdentity(p, o); e != nil {
		return e
	}
	if o.Status != 2 && o.Status != 4 {
		return fail(409, "官方金币充值尚未处于可入账状态，请同步原付款单")
	}
	if o.WXOrderID == "" || o.PaidTime <= 0 {
		return fail(409, "官方金币充值支付凭据尚未齐全，请稍后同步原付款单")
	}
	if time.Unix(o.PaidTime, 0).After(time.Now().Add(5 * time.Minute)) {
		return fail(409, "官方充值支付时间无效")
	}
	return nil
}

// syncVirtualTopup 通过服务器查单核实原现金充值，未支付保持原意图，已退款或状态冲突冻结后续金币消费。
func (a *App) syncVirtualTopup(ctx context.Context, p WalletTopup) error {
	var result struct {
		Order virtualCashOrder `json:"order"`
	}
	code, e := a.virtualRequest(ctx, p.AppID, p.OfferID, p.VirtualEnv, "/xpay/query_order", map[string]any{"openid": p.PayOpenID, "env": p.VirtualEnv, "order_id": p.OrderNo}, "", &result)
	if e != nil {
		return e
	}
	if code != 0 {
		return virtualResultError(code)
	}
	// 只记录数值和匹配结果，既能区分金额/状态/凭据问题，也不泄露OpenID、签名或交易凭证。
	log.Printf("金币充值核验 topup=%s local_status=%s expected_cents=%d order_fee=%d paid_fee=%d official_status=%d order_type=%d order_matches=%t receipt_present=%t paid_time=%d left_fee=%d", p.ID, p.Status, p.AmountCents, result.Order.OrderFee, result.Order.PaidFee, result.Order.Status, result.Order.OrderType, result.Order.OrderID == p.OrderNo, result.Order.WXOrderID != "", result.Order.PaidTime, result.Order.LeftFee)
	if result.Order.OrderID != p.OrderNo {
		return fail(409, "官方充值查询返回了不同原单")
	}
	if result.Order.Status == 5 || result.Order.Status == 8 || p.Status == "paid" && result.Order.LeftFee > 0 && result.Order.LeftFee < p.AmountCents {
		if e = validateVirtualCashIdentity(p, result.Order); e != nil {
			return e
		}
		if result.Order.LeftFee < 0 || result.Order.LeftFee >= p.AmountCents {
			return fail(409, "官方退款状态与可退金额不一致，请平台核账")
		}
		amount := p.AmountCents - result.Order.LeftFee
		if e = a.recordVirtualCashRefund(ctx, p, amount, "query:"+p.ID+":"+strconv.Itoa(result.Order.Status)+":"+strconv.FormatInt(amount, 10), true); e != nil {
			return e
		}
		if p.Status == "paid" {
			return fail(409, "官方金币现金订单已退款，相关来源已冻结待平台核账")
		}
		return nil
	}
	if p.Status == "paid" && result.Order.Status != 2 && result.Order.Status != 4 {
		return a.freezeVirtualSource(ctx, p)
	}
	// 0/1未付、3官方处理中、6关闭均不能产生入账；保留同一原单等待下一次可信查询。
	if result.Order.Status == 0 || result.Order.Status == 1 || result.Order.Status == 3 || result.Order.Status == 6 {
		return nil
	}
	if e = validateVirtualCashIdentity(p, result.Order); e != nil {
		return e
	}
	// 上游已付状态但支付凭据尚未传播完整时继续等待；金额和类型仍必须先核对一致。
	if p.Status == "pending_payment" && (result.Order.Status == 2 || result.Order.Status == 4) && (result.Order.WXOrderID == "" || result.Order.PaidTime == 0) {
		return nil
	}
	if e = validateVirtualCashOrder(p, result.Order); e != nil {
		return e
	}

	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = lockWalletUsers(ctx, tx, p.UserID); e != nil {
		return e
	}
	current, e := get[WalletTopup](ctx, tx, "wallet_topups", p.ID, true)
	if e != nil {
		return e
	}
	if current.OrderNo != p.OrderNo || current.AmountCents != p.AmountCents || current.Channel != "virtual" || current.AppID != p.AppID || current.PayOpenID != p.PayOpenID || current.OfferID != p.OfferID || current.VirtualEnv != p.VirtualEnv {
		return fail(409, "金币充值原单发生冲突")
	}
	evidence := virtualTransaction{ID: p.ID, TopupID: p.ID, Order: result.Order, CheckedAt: now()}
	previous, pe := get[virtualTransaction](ctx, tx, "virtual_transactions", p.ID, false)
	if pe == nil {
		evidence.ReconciliationRequired = previous.ReconciliationRequired
		evidence.CashRefundCents = previous.CashRefundCents
		evidence.NotifiedRefundCents = previous.NotifiedRefundCents
		evidence.RecoveredCoinCents = previous.RecoveredCoinCents
	} else if !missing(pe) {
		return pe
	}
	if e = put(ctx, tx, "virtual_transactions", p.ID, evidence); e != nil {
		return e
	}
	if current.Status == "paid" {
		if current.TransactionID != result.Order.WXOrderID {
			return fail(409, "金币充值官方交易号冲突")
		}
		return tx.Commit(ctx)
	}
	if current.Status != "pending_payment" {
		return fail(409, "金币充值状态异常，请联系平台核账")
	}
	wallet, e := lockedWallet(ctx, tx, p.UserID)
	if e != nil {
		return e
	}
	if e = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "virtual_topup", ReferenceID: p.ID, AmountCents: p.AmountCents, VirtualDelta: p.AmountCents, Note: "微信官方金币充值，1金币=1元，充值不可提现"}); e != nil {
		return e
	}
	current.Status = "paid"
	current.TransactionID = result.Order.WXOrderID
	current.UpdatedAt = now()
	if e = put(ctx, tx, "wallet_topups", p.ID, current); e != nil {
		return e
	}
	if e = addFundingRoot(ctx, tx, "topup", p.ID, current.TransactionID, current.AmountCents, "virtual", p.UserID, "virtual_recharge", "", current.MerchantID, current.AppID); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// syncPendingVirtualTopups 只恢复本人尚未核实的现金充值，避免客户端成功回调丢失后重复买币。
func (a *App) syncPendingVirtualTopups(ctx context.Context, userID string) error {
	items, e := list[WalletTopup](ctx, a.DB, "wallet_topups", "WHERE data->>'user_id'=$1 AND data->>'channel'='virtual' AND data->>'status'='pending_payment' ORDER BY data->>'created_at' LIMIT 20", userID)
	if e != nil {
		return e
	}
	for _, p := range items {
		if e = a.syncVirtualTopup(ctx, p); e != nil {
			return e
		}
	}
	return nil
}
