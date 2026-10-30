// Package app 以官方扣币逆操作退回金币，独立保存退款核验凭据，不将服务退款冒充Apple现金退款。
package app

import (
	"context"
	"strings"
)

// virtualRefundEvidence 保存官方退币成功与手续费退还面额，只有该证据能允许本地恢复金币。
type virtualRefundEvidence struct {
	ID            string `json:"id"`
	RefundID      string `json:"refund_id"`
	ConsumptionID string `json:"consumption_id"`
	OrderNo       string `json:"order_no"`
	AmountCents   int64  `json:"amount_cents"`
	FeeCents      int64  `json:"fee_cents"`
	Status        string `json:"status"`
	ConfirmedAt   string `json:"confirmed_at"`
}

// processVirtualPartRefund 对同一原扣币单执行稳定退币号，超时只重试原号，全退时返还该笔全部额外服务费。
func (a *App) processVirtualPartRefund(ctx context.Context, p OrderPayment, r OrderPartRefund) error {
	c, e := get[virtualConsumption](ctx, a.DB, "virtual_consumptions", virtualConsumptionID(p.OrderID, p.AdjustmentID), false)
	if e != nil {
		return e
	}
	if c.Status != "paid" || c.UserID != p.BuyerID || c.OrderNo != p.TransactionID || c.Quote.ServiceCents != p.AmountCents || r.AmountCents <= 0 || r.AmountCents%100 != 0 {
		return fail(409, "退币金额或原官方扣币凭据无法核对，未改变退款金额")
	}
	fee := int64(0)
	if p.RefundedCents+r.AmountCents == p.AmountCents {
		fee = c.Quote.FeeCents
	}
	id := walletKey("virtual-refund", r.ID)
	proof, e := get[virtualRefundEvidence](ctx, a.DB, "virtual_transactions", id, false)
	if missing(e) {
		for _, source := range r.Sources {
			lot, le := get[FundingLot](ctx, a.DB, "fund_lots", source.LotID, false)
			if le != nil {
				return le
			}
			if le = verifyFundingLot(ctx, a.DB, lot, false, map[string]bool{}); le != nil {
				return le
			}
		}
		s, e := a.miniProgramSession(ctx, c.UserID)
		if e != nil {
			return e
		}
		if s.AppID != c.AppID || s.OpenID != c.OpenID {
			return fail(409, "请原付款用户刷新小程序支付会话后同步退币")
		}
		orderNo := "VR" + strings.ToUpper(id[:30])
		code, e := a.virtualRequest(ctx, c.AppID, c.OfferID, c.Env, "/xpay/cancel_currency_pay", map[string]any{"openid": c.OpenID, "env": c.Env, "user_ip": c.UserIP, "pay_order_id": c.OrderNo, "order_id": orderNo, "amount": (r.AmountCents + fee) / 100}, s.SessionKey, nil)
		if e != nil {
			return e
		}
		if e = virtualResultError(code); e != nil {
			return e
		}
		proof = virtualRefundEvidence{ID: id, RefundID: r.ID, ConsumptionID: c.ID, OrderNo: orderNo, AmountCents: r.AmountCents, FeeCents: fee, Status: "success", ConfirmedAt: now()}
		if e = put(ctx, a.DB, "virtual_transactions", id, proof); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}
	if proof.Status != "success" || proof.RefundID != r.ID || proof.AmountCents != r.AmountCents || proof.FeeCents != fee {
		return fail(409, "官方退币凭据冲突")
	}
	v := wxRefund{OrderNo: p.OrderNo, OutRefundNo: r.RefundNo, TransactionID: p.TransactionID, Status: "SUCCESS", SuccessTime: proof.ConfirmedAt}
	v.Amount.Total = p.AmountCents
	v.Amount.Refund = r.AmountCents
	v.Amount.Currency = "CNY"
	return a.applyOrderPartRefund(ctx, wxPayConfig{}, v, true)
}

// applyVirtualRefundBalance 仅在已核验退币的事务内恢复服务金币和原服务费来源，充值款始终不可提现。
func applyVirtualRefundBalance(ctx context.Context, q querier, p OrderPayment, r OrderPartRefund) error {
	proof, e := get[virtualRefundEvidence](ctx, q, "virtual_transactions", walletKey("virtual-refund", r.ID), false)
	if e != nil {
		return e
	}
	if proof.Status != "success" || proof.RefundID != r.ID || proof.AmountCents != r.AmountCents || proof.ConsumptionID != virtualConsumptionID(p.OrderID, p.AdjustmentID) {
		return fail(409, "缺少官方成功退币凭据")
	}
	if proof.FeeCents > 0 {
		if e = moveFunding(ctx, q, "virtual-fee-refund:"+r.ID, "virtual-fee:"+proof.ConsumptionID, "virtual_fee", proof.FeeCents, []fundDestination{{AccountID: p.BuyerID, Bucket: "virtual_recharge", AmountCents: proof.FeeCents}}); e != nil {
			return e
		}
	}
	wallet, e := lockedWallet(ctx, q, p.BuyerID)
	if e != nil {
		return e
	}
	return appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "virtual_refund", ReferenceID: r.ID, AmountCents: r.AmountCents + proof.FeeCents, VirtualDelta: r.AmountCents + proof.FeeCents, Note: "官方服务退币；本笔服务全退时同时返还原额外金币服务费"})
}
