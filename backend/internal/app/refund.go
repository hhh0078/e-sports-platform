// Package app 通过先记录退款意图再调用外部通道，保证网络超时不会丢失资金状态。
package app

import (
	"context"
	"log"
)

// prepareRemoteRefund 在订单事务内冻结履约并持久化稳定退款号，尚不调用外部接口。
func (a *App) prepareRemoteRefund(o *Order, before Order) error {
	if _, err := a.loadWXPay(); err != nil {
		return err
	}
	if before.PaymentTransactionID == "" {
		return fail(409, "缺少真实支付流水，不能执行真实退款")
	}
	o.RefundID = "RF" + o.ID
	o.Status = before.Status
	o.PaymentStatus = before.PaymentStatus
	o.RefundStatus = "processing"
	o.RefundReply = "退款申请已记录，等待微信确认资金结果"
	o.UpdatedAt = now()
	return nil
}

// submitRemoteRefund 必须在退款意图事务提交后调用，未知结果保留冻结并由查询补偿。
func (a *App) submitRemoteRefund(ctx context.Context, o Order, reason string) (Order, error) {
	done, err := a.refund(ctx, o, reason)
	if err != nil {
		log.Printf("退款资金状态待同步 order=%s: %v", o.ID, err)
		return o, nil
	}
	if done {
		c, e := a.loadWXPay()
		if e != nil {
			return o, e
		}
		v := wxRefund{MerchantID: c.MerchantID, OrderNo: o.OrderNo, OutRefundNo: o.RefundID, TransactionID: o.PaymentTransactionID, Status: "SUCCESS"}
		v.Amount.Total = o.AmountCents
		v.Amount.Refund = o.AmountCents
		v.Amount.Currency = "CNY"
		if e = a.applyWXRefund(ctx, c, v); e != nil {
			return o, e
		}
	}
	return get[Order](ctx, a.DB, "orders", o.ID, false)
}
