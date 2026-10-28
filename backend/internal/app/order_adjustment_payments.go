// Package app 通过独立补款单完成加价，微信回调与原单查询均核验原金额和付款人。
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// adjustmentPaymentID 为一个方案保留唯一补款原单，不因请求超时更换单号。
func adjustmentPaymentID(id string) string { return walletKey("adjustment-payment", id) }

// adjustOrderNet 使用原冻结比例计算累计净价，差额分录避免小额反复调价的舍入套利。
func adjustOrderNet(ctx context.Context, q querier, o *Order, reference string, delta int64) error {
	amount, e := checkedWalletAmount(o.AmountCents, delta)
	if e != nil || amount <= 0 || amount > 10000000 {
		return fail(409, "调整后订单金额无效")
	}
	old := settlementForOrder(*o)
	next := old
	if next.Version == 0 {
		next.Version = 1
	}
	next.PlatformFeeCents = amount * old.PlatformRateBPS / 10000
	next.ClubFeeCents = amount * old.ClubRateBPS / 10000
	next.ProviderIncomeCents = amount - next.PlatformFeeCents - next.ClubFeeCents
	if next.ProviderIncomeCents < 0 {
		return fail(409, "原订单分账比例无效")
	}
	for _, part := range []struct {
		account string
		delta   int64
	}{{o.ProviderID, next.ProviderIncomeCents - old.ProviderIncomeCents}, {clubWalletID(o.ClubID), next.ClubFeeCents - old.ClubFeeCents}} {
		if part.delta == 0 {
			continue
		}
		wallet, e := lockedWallet(ctx, q, part.account)
		if e != nil {
			return e
		}
		if e = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_price_adjustment", ReferenceID: reference, AmountCents: part.delta, PendingDelta: part.delta, Note: "双方确认调价，按原订单费率调整待结算款"}); e != nil {
			return e
		}
	}
	escrow, e := get[WalletEscrow](ctx, q, "wallet_escrows", o.ID, true)
	if e != nil {
		return e
	}
	if escrow.Status != "holding" || escrow.AmountCents != o.AmountCents {
		return fail(409, "订单托管已变化，不能调整净价")
	}
	escrow.AmountCents = amount
	o.AmountCents = amount
	o.Settlement = next
	o.UpdatedAt = now()
	return put(ctx, q, "wallet_escrows", o.ID, escrow)
}

// accountAdjustmentPayment 在资金成功事务中追加独立来源和原比例收入，履约阶段保持不变。
func accountAdjustmentPayment(ctx context.Context, q querier, o *Order, v *OrderAdjustment, p *OrderPayment, channel string) error {
	if p.Status == "paid" {
		return nil
	}
	if o.ActiveAdjustmentID != v.ID || v.Kind != "increase" || v.Status != "paying" || !v.BuyerAgreed || !v.ProviderAgreed {
		return fail(409, "补款与当前双方确认方案不一致")
	}
	if e := adjustmentEligible(*o, "increase"); e != nil {
		return e
	}
	if channel == "balance" {
		wallet, e := lockedWallet(ctx, q, o.BuyerID)
		if e != nil {
			return e
		}
		spendable, e := checkedWalletAmount(wallet.RechargeCents, wallet.AvailableCents)
		if e != nil {
			return e
		}
		if spendable < p.AmountCents {
			return fail(409, "可消费余额不足")
		}
		p.RechargeUsed = p.AmountCents
		if p.RechargeUsed > wallet.RechargeCents {
			p.RechargeUsed = wallet.RechargeCents
		}
		p.IncomeUsed = p.AmountCents - p.RechargeUsed
		if e = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_adjustment_payment", ReferenceID: p.ID, AmountCents: -p.AmountCents, RechargeDelta: -p.RechargeUsed, AvailableDelta: -p.IncomeUsed, Note: "余额支付双方确认的订单补款"}); e != nil {
			return e
		}
		for _, part := range []struct {
			bucket string
			amount int64
		}{{"recharge", p.RechargeUsed}, {"available", p.IncomeUsed}} {
			if part.amount > 0 {
				if e = moveFunding(ctx, q, "adjustment-pay:"+p.ID+":"+part.bucket, o.BuyerID, part.bucket, part.amount, []fundDestination{{AccountID: "escrow:" + o.ID, Bucket: "escrow", OrderID: o.ID, AmountCents: part.amount, PreserveReturn: true}}); e != nil {
					return e
				}
			}
		}
	} else {
		if e := addFundingRoot(ctx, q, "order_payment", p.ID, p.TransactionID, p.AmountCents, channel, "escrow:"+o.ID, "escrow", o.ID, p.MerchantID, p.AppID); e != nil {
			return e
		}
	}
	if e := adjustOrderNet(ctx, q, o, v.ID, p.AmountCents); e != nil {
		return e
	}
	p.Channel = channel
	p.Status = "paid"
	if p.PaidAt == "" {
		p.PaidAt = now()
	}
	p.UpdatedAt = now()
	o.PaidTotalCents += p.AmountCents
	o.ActiveAdjustmentID = ""
	v.Status = "completed"
	v.CompletedAt = p.PaidAt
	v.UpdatedAt = now()
	if e := put(ctx, q, "order_payments", p.ID, *p); e != nil {
		return e
	}
	if e := put(ctx, q, "order_adjustments", v.ID, *v); e != nil {
		return e
	}
	if e := put(ctx, q, "orders", o.ID, *o); e != nil {
		return e
	}
	return systemMessage(ctx, q, *o, fmt.Sprintf("补款 ¥%.2f 已核验成功，当前订单净价 ¥%.2f。平台及俱乐部仍按原订单比例分账。", float64(p.AmountCents)/100, float64(o.AmountCents)/100))
}

// payOrderAdjustment 只允许买家支付已双方确认的加价，并在出网前提交付款意图。
func (a *App) payOrderAdjustment(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	var in struct {
		Channel   string `json:"channel"`
		RequestID string `json:"request_id"`
	}
	if e = decode(r, &in); e != nil {
		return nil, e
	}
	if in.Channel == "" {
		in.Channel = "wechat"
	}
	if !contains([]string{"wechat", "balance"}, in.Channel) {
		return nil, fail(400, "支付渠道无效")
	}
	if in.Channel == "balance" {
		if e = walletRequestID(in.RequestID); e != nil {
			return nil, e
		}
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(r.Context())
	o, e := lockWalletOrder(r.Context(), tx, r.PathValue("id"), "")
	if e != nil {
		return nil, e
	}
	if u.ID != o.BuyerID {
		return nil, fail(403, "只有买家可以支付差额")
	}
	v, e := get[OrderAdjustment](r.Context(), tx, "order_adjustments", r.PathValue("adjustment_id"), true)
	if e != nil {
		return nil, e
	}
	if v.OrderID != o.ID || v.Kind != "increase" {
		return nil, fail(404, "补款方案不存在")
	}
	if v.Status == "completed" {
		return map[string]any{"adjustment": v, "order": o, "mode": paymentMode(a.Config.DevMode)}, nil
	}
	if e = requireAdultIdentity(r.Context(), tx, u.ID); e != nil {
		return nil, e
	}
	if !contains([]string{"awaiting_payment", "paying"}, v.Status) || !v.BuyerAgreed || !v.ProviderAgreed || o.ActiveAdjustmentID != v.ID {
		return nil, fail(409, "方案尚未双方同意、已过期或资金状态不符")
	}
	if expires, parseErr := time.Parse(time.RFC3339Nano, v.ExpiresAt); parseErr != nil || !time.Now().Before(expires) {
		return nil, fail(409, "补款已到期，请先同步原单资金及关闭状态后重新协商")
	}
	if e = adjustmentEligible(o, "increase"); e != nil {
		return nil, e
	}
	if e = guardDepositRefunding(r.Context(), tx, o.BuyerID, o.ProviderID); e != nil {
		return nil, e
	}
	p, e := get[OrderPayment](r.Context(), tx, "order_payments", adjustmentPaymentID(v.ID), true)
	if missing(e) {
		p = OrderPayment{ID: adjustmentPaymentID(v.ID), OrderID: o.ID, AdjustmentID: v.ID, BuyerID: o.BuyerID, Kind: "supplement", Channel: in.Channel, AmountCents: v.AmountCents, Status: "pending_payment", OrderNo: "OA" + strings.ToUpper(walletKey(v.ID)[:30]), RequestID: in.RequestID, CreatedAt: now(), UpdatedAt: now()}
		e = nil
	} else if e == nil && (p.Channel != in.Channel && !(a.Config.DevMode && p.Channel == "dev" && in.Channel == "wechat") || in.Channel == "balance" && p.RequestID != in.RequestID) {
		return nil, fail(409, "已存在其他支付渠道或请求号的补款，请先同步原单")
	}
	if e != nil {
		return nil, e
	}
	v.Status = "paying"
	v.UpdatedAt = now()
	if in.Channel == "balance" {
		if e = claimWalletRequest(r.Context(), tx, u.ID, in.RequestID, "order_adjustment_payment", map[string]any{"adjustment_id": v.ID, "amount_cents": p.AmountCents}); e != nil {
			return nil, e
		}
		if e = accountAdjustmentPayment(r.Context(), tx, &o, &v, &p, "balance"); e != nil {
			return nil, e
		}
		return map[string]any{"adjustment": v, "order": o, "mode": "balance"}, tx.Commit(r.Context())
	}
	if a.Config.DevMode {
		p.Channel = "dev"
		p.TransactionID = "DEVADJUST" + p.ID
		if e = accountAdjustmentPayment(r.Context(), tx, &o, &v, &p, "dev"); e != nil {
			return nil, e
		}
		return map[string]any{"adjustment": v, "order": o, "mode": "dev"}, tx.Commit(r.Context())
	}
	c, e := a.loadWXPay()
	if e != nil {
		return nil, e
	}
	expires, parseErr := time.Parse(time.RFC3339Nano, v.ExpiresAt)
	if parseErr != nil || time.Until(expires) < 5*time.Minute {
		return nil, fail(409, "补款方案临近到期，请等待到期并同步原单后重新协商")
	}
	if creating, parseErr := time.Parse(time.RFC3339Nano, v.PaymentCreatingUntil); parseErr == nil && time.Now().Before(creating) {
		return nil, fail(409, "补款支付会话正在创建，请稍候同步原单")
	}
	v.PaymentCreatingUntil = time.Now().Add(2 * time.Minute).UTC().Format(time.RFC3339Nano)
	identity, e := get[map[string]string](r.Context(), tx, "identities", u.ID, false)
	if e != nil || identity["openid"] == "" {
		return nil, fail(409, "请使用微信登录后补款")
	}
	if p.PayOpenID != "" && (p.PayOpenID != identity["openid"] || p.AppID != c.AppID || p.MerchantID != c.MerchantID) {
		return nil, fail(409, "原补款商户或付款身份已变化，请同步原单")
	}
	p.PayOpenID = identity["openid"]
	p.MerchantID = c.MerchantID
	p.AppID = c.AppID
	if p.ExpiresAt == "" {
		p.ExpiresAt = v.ExpiresAt
	}
	if e = put(r.Context(), tx, "order_payments", p.ID, p); e != nil {
		return nil, e
	}
	if e = put(r.Context(), tx, "order_adjustments", v.ID, v); e != nil {
		return nil, e
	}
	if e = tx.Commit(r.Context()); e != nil {
		return nil, e
	}
	params, e := a.prepayOrderPart(r.Context(), c, p, o.Title, p.ExpiresAt)
	current, clearErr := a.finishAdjustmentCreation(context.WithoutCancel(r.Context()), o.ID, v.ID, v.PaymentCreatingUntil)
	if clearErr != nil {
		return nil, clearErr
	}
	if e != nil {
		return nil, e
	}
	if current.Status != "paying" {
		return nil, fail(409, "调价方案已结束，如原单实际收到补款将自动原路退还")
	}
	if deadline, parseErr := time.Parse(time.RFC3339Nano, current.ExpiresAt); parseErr != nil || !time.Now().Before(deadline) {
		return nil, fail(409, "调价已到期，请同步原支付状态，勿再次付款")
	}
	return map[string]any{"adjustment": v, "order": o, "mode": "wechat", "pay_params": params}, nil
}

// prepayOrderPart 为稳定补款原单生成微信支付参数，不持有任何数据库资金锁。
func (a *App) prepayOrderPart(ctx context.Context, c wxPayConfig, p OrderPayment, title, expiry string) (map[string]string, error) {
	if p.MerchantID != c.MerchantID || p.AppID != c.AppID {
		return nil, fail(409, "补款商户配置与原单不一致")
	}
	var result struct {
		PrepayID string `json:"prepay_id"`
	}
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	deadline, parseErr := time.Parse(time.RFC3339Nano, expiry)
	if parseErr != nil {
		return nil, fail(409, "原补款固定有效期无效")
	}
	if e := a.wxRequest(requestContext, c, "POST", "/v3/pay/transactions/jsapi", map[string]any{"appid": p.AppID, "mchid": p.MerchantID, "description": "订单补款 " + title, "out_trade_no": p.OrderNo, "time_expire": deadline.Format(time.RFC3339), "notify_url": c.NotifyURL, "amount": map[string]any{"total": p.AmountCents, "currency": "CNY"}, "payer": map[string]string{"openid": p.PayOpenID}}, &result); e != nil {
		return nil, e
	}
	if result.PrepayID == "" {
		return nil, fail(502, "微信未返回补款支付会话")
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := newID()
	pack := "prepay_id=" + result.PrepayID
	sign, e := wxSign(c.PrivateKey, c.AppID+"\n"+stamp+"\n"+nonce+"\n"+pack+"\n")
	if e != nil {
		return nil, e
	}
	return map[string]string{"timeStamp": stamp, "nonceStr": nonce, "package": pack, "signType": "RSA", "paySign": sign}, nil
}

// applyAdjustmentWXPayment 严格核对补款原单；重复通知不重复增加收入。
func (a *App) applyAdjustmentWXPayment(ctx context.Context, c wxPayConfig, v wxTransaction) error {
	var id string
	if e := a.DB.QueryRow(ctx, "SELECT id FROM order_payments WHERE data->>'order_no'=$1", v.OrderNo).Scan(&id); e != nil {
		return fail(404, "补款原单不存在")
	}
	snapshot, e := get[OrderPayment](ctx, a.DB, "order_payments", id, false)
	if e != nil {
		return e
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	o, e := lockWalletOrder(ctx, tx, snapshot.OrderID, "")
	if e != nil {
		return e
	}
	p, e := get[OrderPayment](ctx, tx, "order_payments", id, true)
	if e != nil {
		return e
	}
	if v.MerchantID != c.MerchantID || v.AppID != c.AppID || v.MerchantID != p.MerchantID || v.AppID != p.AppID || v.Amount.Total != p.AmountCents || v.Amount.Currency != "CNY" || v.Payer.OpenID != p.PayOpenID || v.TradeState != "SUCCESS" || v.TransactionID == "" {
		return fail(400, "补款金额、商户或付款人不匹配")
	}
	if p.Status == "paid" {
		if p.TransactionID == v.TransactionID {
			return nil
		}
		return fail(409, "补款支付流水冲突")
	}
	p.TransactionID = v.TransactionID
	p.PaidAt = v.SuccessTime
	if p.PaidAt != "" {
		stamp, parseErr := time.Parse(time.RFC3339Nano, p.PaidAt)
		if parseErr != nil {
			return fail(400, "微信补款时间无效")
		}
		p.PaidAt = stamp.UTC().Format(time.RFC3339Nano)
	}
	adjustment, e := get[OrderAdjustment](ctx, tx, "order_adjustments", p.AdjustmentID, true)
	if e != nil {
		return e
	}
	if o.ActiveAdjustmentID != adjustment.ID || adjustment.Status != "paying" || !contains([]string{"open", "accepted", "in_progress", "delivered"}, o.Status) || o.RefundStatus == "processing" {
		if e = accountLateAdjustmentPayment(ctx, tx, o, &adjustment, &p); e != nil {
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
		_ = a.processOrderRefunds(ctx, o.ID, "late:"+p.ID)
		return nil
	}
	if e = accountAdjustmentPayment(ctx, tx, &o, &adjustment, &p, "wechat"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// syncOrderAdjustment 由买家或平台补偿原补款和退款，过期未付原单先关闭后解除冻结。
func (a *App) syncOrderAdjustment(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	o, e := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if e != nil {
		return nil, e
	}
	if u.ID != o.BuyerID && u.Role != "admin" {
		return nil, fail(403, "仅买家或平台可同步调价资金")
	}
	v, e := get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", r.PathValue("adjustment_id"), false)
	if e != nil {
		return nil, e
	}
	if v.OrderID != o.ID {
		return nil, fail(404, "调价方案不存在")
	}
	if v.Status == "refunding" {
		group := v.ID
		p, readErr := get[OrderPayment](r.Context(), a.DB, "order_payments", adjustmentPaymentID(v.ID), false)
		if readErr == nil && p.Kind == "late_supplement" {
			group = "late:" + p.ID
		} else if readErr != nil && !missing(readErr) {
			return nil, readErr
		}
		if e = a.processOrderRefunds(r.Context(), o.ID, group); e != nil {
			return nil, e
		}
	} else if v.Status == "paying" && !a.Config.DevMode {
		if expires, parseErr := time.Parse(time.RFC3339Nano, v.ExpiresAt); parseErr == nil && !time.Now().Before(expires) && adjustmentCreationActive(v) {
			return nil, fail(409, "原补款创建仍在处理中，请稍后同步，当前不会解冻或更换付款单")
		}
		p, e := get[OrderPayment](r.Context(), a.DB, "order_payments", adjustmentPaymentID(v.ID), false)
		if e != nil {
			return nil, e
		}
		c, e := a.loadWXPay()
		if e != nil {
			return nil, e
		}
		if p.AppID != c.AppID || p.MerchantID != c.MerchantID {
			return nil, fail(409, "补款商户与当前配置不一致，请恢复原商户核账")
		}
		var result wxTransaction
		e = a.wxRequest(r.Context(), c, "GET", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(p.OrderNo)+"?mchid="+url.QueryEscape(c.MerchantID), nil, &result)
		if e != nil {
			if wxAdjustmentOrderMissing(e) {
				expires, parseErr := time.Parse(time.RFC3339Nano, v.ExpiresAt)
				if parseErr == nil && !time.Now().Before(expires) {
					closeErr := a.wxRequest(r.Context(), c, "POST", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(p.OrderNo)+"/close", map[string]string{"mchid": c.MerchantID}, nil)
					if closeErr == nil || wxAdjustmentOrderMissing(closeErr) {
						if closeErr = a.expireClosedAdjustment(r.Context(), o.ID, v.ID); closeErr != nil {
							return nil, closeErr
						}
						return get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", v.ID, false)
					}
				}
			}
			return nil, e
		}
		if result.OrderNo != p.OrderNo || result.MerchantID != p.MerchantID || result.AppID != p.AppID || result.Amount.Total != p.AmountCents {
			return nil, fail(400, "补款查询结果与原单不匹配")
		}
		if result.TradeState == "SUCCESS" {
			e = a.applyAdjustmentWXPayment(r.Context(), c, result)
		} else {
			expiry, pe := time.Parse(time.RFC3339Nano, v.ExpiresAt)
			if pe != nil {
				return nil, pe
			}
			if result.TradeState == "CLOSED" || !time.Now().Before(expiry) {
				if result.TradeState != "CLOSED" {
					e = a.wxRequest(r.Context(), c, "POST", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(p.OrderNo)+"/close", map[string]string{"mchid": c.MerchantID}, nil)
				}
				if e == nil {
					e = a.expireClosedAdjustment(r.Context(), o.ID, v.ID)
				}
			}
		}
		if e != nil {
			return nil, e
		}
	}
	return get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", v.ID, false)
}

// wxAdjustmentOrderMissing 仅识别经验签的微信业务不存在，网络错误或无签名404仍为未知资金结果。
func wxAdjustmentOrderMissing(err error) bool {
	var upstream *wxAPIError
	return errors.As(err, &upstream) && contains([]string{"ORDER_NOT_EXIST", "RESOURCE_NOT_EXISTS", "NOT_FOUND"}, upstream.Code)
}

// expireClosedAdjustment 仅在官方原单已关闭后终止待补款，不能凭客户端取消结果操作。
func (a *App) expireClosedAdjustment(ctx context.Context, orderID, id string) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	o, e := lockWalletOrder(ctx, tx, orderID, "")
	if e != nil {
		return e
	}
	v, e := get[OrderAdjustment](ctx, tx, "order_adjustments", id, true)
	if e != nil {
		return e
	}
	if v.Status != "paying" {
		return nil
	}
	if adjustmentCreationActive(v) {
		return fail(409, "补款创建仍在处理中，不能关闭调价")
	}
	v.Status = "expired"
	v.UpdatedAt = now()
	o.ActiveAdjustmentID = ""
	if e = put(ctx, tx, "order_adjustments", id, v); e != nil {
		return e
	}
	if e = put(ctx, tx, "orders", o.ID, o); e != nil {
		return e
	}
	if e = systemMessage(ctx, tx, o, "微信已确认补款原单关闭，调价失效，原订单金额不变。"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
