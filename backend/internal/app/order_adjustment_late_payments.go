// Package app 为补款创建保留有限在途租约，并对任何已确认迟到实付隔离入账后原路退回。
package app

import (
	"context"
	"strings"
	"time"
)

// adjustmentCreationActive 判断创建租约是否仍有效，崩溃残留不会永久锁死协商。
func adjustmentCreationActive(v OrderAdjustment) bool {
	deadline, e := time.Parse(time.RFC3339Nano, v.PaymentCreatingUntil)
	return e == nil && time.Now().Before(deadline)
}

// finishAdjustmentCreation 在独立短事务清除出网租约，客户端断线也保留真实原单并允许后续恢复。
func (a *App) finishAdjustmentCreation(parent context.Context, orderID, id, lease string) (OrderAdjustment, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return OrderAdjustment{}, e
	}
	defer tx.Rollback(ctx)
	_, e = lockWalletOrder(ctx, tx, orderID, "")
	if e != nil {
		return OrderAdjustment{}, e
	}
	v, e := get[OrderAdjustment](ctx, tx, "order_adjustments", id, true)
	if e != nil {
		return v, e
	}
	if v.PaymentCreatingUntil != lease {
		return v, tx.Commit(ctx)
	}
	v.PaymentCreatingUntil = ""
	v.UpdatedAt = now()
	if e = put(ctx, tx, "order_adjustments", id, v); e != nil {
		return v, e
	}
	return v, tx.Commit(ctx)
}

// accountLateAdjustmentPayment 不丢弃任何鉴真的成功收款，隔离款项不进入订单净价、GMV或三方收入。
func accountLateAdjustmentPayment(ctx context.Context, q querier, o Order, v *OrderAdjustment, p *OrderPayment) error {
	if p.Status == "paid" {
		return nil
	}
	p.Kind = "late_supplement"
	p.Status = "paid"
	if p.PaidAt == "" {
		p.PaidAt = now()
	}
	p.UpdatedAt = now()
	p.ReservedCents = p.AmountCents
	account := "overpayment:" + p.ID
	if e := addFundingRoot(ctx, q, "order_payment", p.ID, p.TransactionID, p.AmountCents, p.Channel, account, "overpayment", o.ID, p.MerchantID, p.AppID); e != nil {
		return e
	}
	lots, e := fundingLots(ctx, q, account, "overpayment")
	if e != nil {
		return e
	}
	sources := []FundingPart{}
	var total int64
	for _, l := range lots {
		sources = append(sources, FundingPart{l.ID, l.RemainingCents})
		total += l.RemainingCents
	}
	if total != p.AmountCents {
		return fail(409, "迟到补款隔离来源不足")
	}
	group := "late:" + p.ID
	r := OrderPartRefund{ID: walletKey("late-order-refund", p.ID), Purpose: "late", OrderID: o.ID, AdjustmentID: v.ID, GroupID: group, PaymentID: p.ID, RefundNo: "OR" + strings.ToUpper(walletKey(group, p.ID)[:60]), AmountCents: p.AmountCents, Status: "queued", Reason: "调价已结束，迟到补款原路退还", Sources: sources, CreatedAt: now(), UpdatedAt: now()}
	v.Status = "refunding"
	v.FailureReason = "迟到补款未计入订单费用，正在按原支付渠道退还"
	v.PaymentCreatingUntil = ""
	v.UpdatedAt = now()
	if e = put(ctx, q, "order_payments", p.ID, *p); e != nil {
		return e
	}
	if e = put(ctx, q, "order_refunds", r.ID, r); e != nil {
		return e
	}
	if e = put(ctx, q, "order_adjustments", v.ID, *v); e != nil {
		return e
	}
	return systemMessage(ctx, q, o, "平台已核验收到已结束调价的迟到补款，已单独登记并发起原路退还；原订单金额及三方收入不变。")
}

// finishLateOrderRefund 仅结束隔离退款，不覆盖同一订单后来发生的新协商或履约状态。
func finishLateOrderRefund(ctx context.Context, q querier, o Order, r OrderPartRefund) error {
	v, e := get[OrderAdjustment](ctx, q, "order_adjustments", r.AdjustmentID, true)
	if e != nil {
		return e
	}
	v.Status = "refunded"
	v.CompletedAt = r.SuccessAt
	v.UpdatedAt = now()
	v.FailureReason = "迟到补款已全部按原支付渠道退还，未调整订单费用"
	if e = put(ctx, q, "order_adjustments", v.ID, v); e != nil {
		return e
	}
	return systemMessage(ctx, q, o, "迟到补款已全部原路退还；原订单费用及后续履约记录保持不变。")
}
