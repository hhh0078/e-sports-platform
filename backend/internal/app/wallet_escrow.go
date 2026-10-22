// Package app 将真实付款逐单托管，并在售后期结束后把服务收入释放至可提现余额。
package app

import (
	"context"
	"time"
)

// accountWalletPayment 只为新的实付建立一次托管，余额付款保留充值款和收入原始分账。
func accountWalletPayment(ctx context.Context, q querier, o *Order, channel, requestID string) error {
	existing, err := get[WalletEscrow](ctx, q, "wallet_escrows", o.ID, true)
	if err == nil {
		if existing.AmountCents != o.AmountCents || existing.Channel != channel || existing.BuyerID != o.BuyerID || existing.ProviderID != o.ProviderID {
			return fail(409, "订单托管资金来源冲突")
		}
		o.WalletAccounted = true
		return nil
	}
	if !missing(err) {
		return err
	}
	e := WalletEscrow{OrderID: o.ID, BuyerID: o.BuyerID, ProviderID: o.ProviderID, AmountCents: o.AmountCents, Channel: channel, Status: "holding", CreatedAt: now()}
	if channel == "balance" {
		wallet, err := lockedWallet(ctx, q, o.BuyerID)
		if err != nil {
			return err
		}
		spendable, err := checkedWalletAmount(wallet.RechargeCents, wallet.AvailableCents)
		if err != nil {
			return err
		}
		if spendable < o.AmountCents {
			return fail(409, "可消费余额不足，请充值后重试")
		}
		e.RechargeUsed = o.AmountCents
		if e.RechargeUsed > wallet.RechargeCents {
			e.RechargeUsed = wallet.RechargeCents
		}
		e.IncomeUsed = o.AmountCents - e.RechargeUsed
		if err = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_payment", ReferenceID: o.ID, AmountCents: -o.AmountCents, RechargeDelta: -e.RechargeUsed, AvailableDelta: -e.IncomeUsed, Note: "余额支付订单"}); err != nil {
			return err
		}
	}
	if e.ProviderID != "" {
		wallet, err := lockedWallet(ctx, q, e.ProviderID)
		if err != nil {
			return err
		}
		if err = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_pending", ReferenceID: o.ID, AmountCents: o.AmountCents, PendingDelta: o.AmountCents, Note: "订单付款托管，完成后等待7天售后期"}); err != nil {
			return err
		}
	}
	o.PaidAt = now()
	o.PaymentChannel = channel
	o.WalletAccounted = true
	o.BalanceRechargeCents = e.RechargeUsed
	o.BalanceIncomeCents = e.IncomeUsed
	o.WalletRequestID = requestID
	return put(ctx, q, "wallet_escrows", o.ID, e)
}

// claimWalletEscrow 把公开需求已经实付的托管款归入接单人的待结算区。
func claimWalletEscrow(ctx context.Context, q querier, o Order) error {
	e, err := get[WalletEscrow](ctx, q, "wallet_escrows", o.ID, true)
	if missing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if e.Status != "holding" {
		return fail(409, "订单托管已结束")
	}
	if e.ProviderID != "" {
		if e.ProviderID != o.ProviderID {
			return fail(409, "托管接单人冲突")
		}
		return nil
	}
	wallet, err := lockedWallet(ctx, q, o.ProviderID)
	if err != nil {
		return err
	}
	if err = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_pending", ReferenceID: o.ID, AmountCents: o.AmountCents, PendingDelta: o.AmountCents, Note: "订单付款托管，完成后等待7天售后期"}); err != nil {
		return err
	}
	e.ProviderID = o.ProviderID
	return put(ctx, q, "wallet_escrows", o.ID, e)
}

// refundWalletEscrow 退款成功后撤回待结算收入，余额支付严格恢复原先支付的两类余额。
func refundWalletEscrow(ctx context.Context, q querier, o *Order) error {
	if o.RefundedAt == "" {
		o.RefundedAt = now()
	}
	e, err := get[WalletEscrow](ctx, q, "wallet_escrows", o.ID, true)
	if missing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if e.AmountCents != o.AmountCents || e.BuyerID != o.BuyerID || e.ProviderID != o.ProviderID {
		return fail(409, "退款订单与托管资金归属不一致，请平台核账")
	}
	if e.Status == "refunded" {
		return nil
	}
	if e.Status != "holding" {
		return fail(409, "该笔收入已结算，需平台人工核账")
	}
	if e.ProviderID != "" {
		wallet, err := lockedWallet(ctx, q, e.ProviderID)
		if err != nil {
			return err
		}
		if err = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_refund_reversal", ReferenceID: o.ID, AmountCents: -e.AmountCents, PendingDelta: -e.AmountCents, Note: "订单退款，撤回待结算款"}); err != nil {
			return err
		}
	}
	if e.Channel == "balance" {
		wallet, err := lockedWallet(ctx, q, e.BuyerID)
		if err != nil {
			return err
		}
		if err = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_balance_refund", ReferenceID: o.ID, AmountCents: e.AmountCents, RechargeDelta: e.RechargeUsed, AvailableDelta: e.IncomeUsed, Note: "退款恢复原支付余额分账"}); err != nil {
			return err
		}
	}
	e.Status = "refunded"
	return put(ctx, q, "wallet_escrows", o.ID, e)
}

// walletIncomeEligible 使用服务端时间检查完成、售后和最终裁决状态，拒绝仅凭订单显示状态释放收入。
func walletIncomeEligible(o Order, stamp time.Time) bool {
	if o.Status != "completed" || o.PaymentStatus != "paid" || !o.WalletAccounted || o.ProviderID == "" {
		return false
	}
	if contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) {
		return false
	}
	if o.RefundStatus == "rejected" && !o.RefundFinal {
		return false
	}
	completed, err := time.Parse(time.RFC3339Nano, o.CompletedAt)
	return err == nil && !stamp.Before(completed.Add(walletHoldDuration))
}

// releaseWalletIncome 每笔订单按统一用户锁序独立结算，避免并发领取、退款或提现时重复释放。
func (a *App) releaseWalletIncome(ctx context.Context, userID string) error {
	escrows, err := list[WalletEscrow](ctx, a.DB, "wallet_escrows", "WHERE data->>'provider_id'=$1 AND data->>'status'='holding'", userID)
	if err != nil {
		return err
	}
	for _, e := range escrows {
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			return err
		}
		o, err := lockWalletOrder(ctx, tx, e.OrderID, "")
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		current, err := get[WalletEscrow](ctx, tx, "wallet_escrows", e.OrderID, true)
		if err == nil && (current.ProviderID != userID || current.ProviderID != o.ProviderID || current.BuyerID != o.BuyerID || current.AmountCents != o.AmountCents) {
			err = fail(409, "托管资金与订单归属或金额不一致，请平台核账")
		}
		if err == nil && current.Status == "holding" && walletIncomeEligible(o, time.Now()) {
			wallet, readErr := lockedWallet(ctx, tx, userID)
			err = readErr
			if err == nil {
				err = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "order_settlement", ReferenceID: o.ID, AmountCents: o.AmountCents, PendingDelta: -o.AmountCents, AvailableDelta: o.AmountCents, Note: "订单已满7天售后期，服务收入可提现"})
			}
			if err == nil {
				current.Status = "released"
				current.ReleasedAt = now()
				err = put(ctx, tx, "wallet_escrows", o.ID, current)
			}
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
