// Package app 严格核验历史实付退款：微信须官方验签，余额须原付款分录和用途；仅反冲旧账，不补造资金来源。
package app

import "context"

// orderFundingEvidence 查找该订单任一新版来源证据，已损坏的新账也不能降级历史兼容。
func orderFundingEvidence(ctx context.Context, q querier, orderID string) (bool, error) {
	ids := []string{}
	for _, event := range []string{"order-pay:" + orderID + ":recharge", "order-pay:" + orderID + ":available", "settlement:" + orderID, "order-refund:" + orderID} {
		ids = append(ids, walletKey("fund-move", event))
	}
	var evidence bool
	e := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fund_lots WHERE data->>'order_id'=$1 OR data->>'account_id'='escrow:'||$1) OR EXISTS(SELECT 1 FROM fund_receipts WHERE data->>'kind'='order' AND data->>'payment_id'=$1) OR EXISTS(SELECT 1 FROM fund_movements WHERE id=ANY($2::text[]))`, orderID, ids).Scan(&evidence)
	return evidence, e
}

// legacyEscrowRefund 严格限定旧账兼容：官方微信原单成功退款或不可变余额付款分录必须可核对，新版缺来源一律不能走兼容。
func legacyEscrowRefund(ctx context.Context, q querier, o Order, e WalletEscrow, verifiedWX bool) (bool, error) {
	if e.FundingVersion != 0 || !e.LegacySourceGap {
		return false, nil
	}
	if !o.WalletAccounted || o.PaymentChannel != e.Channel {
		return false, fail(409, "历史托管支付通道或入账记录无法核对")
	}
	evidence, err := orderFundingEvidence(ctx, q, o.ID)
	if err != nil {
		return false, err
	}
	if evidence {
		return false, fail(409, "历史托管标记与新版来源证据冲突，请平台核账")
	}
	switch e.Channel {
	case "wechat":
		if !verifiedWX || o.PaymentStatus != "paid" || o.PaymentTransactionID == "" {
			return false, fail(409, "历史微信托管须核验官方原单成功退款")
		}
	case "balance":
		if !contains([]string{"paid", "refunded"}, o.PaymentStatus) || e.RechargeUsed < 0 || e.IncomeUsed < 0 || e.RechargeUsed > e.AmountCents || e.IncomeUsed > e.AmountCents || e.RechargeUsed+e.IncomeUsed != e.AmountCents || o.BalanceRechargeCents != e.RechargeUsed || o.BalanceIncomeCents != e.IncomeUsed {
			return false, fail(409, "历史余额付款分区与订单金额无法核对")
		}
		entry, err := get[WalletEntry](ctx, q, "wallet_entries", walletKey(e.BuyerID, "order_payment", o.ID), false)
		if err != nil || entry.UserID != e.BuyerID || entry.Kind != "order_payment" || entry.ReferenceID != o.ID || entry.AmountCents != -e.AmountCents || entry.RechargeDelta != -e.RechargeUsed || entry.AvailableDelta != -e.IncomeUsed || entry.PendingDelta != 0 || entry.WithdrawalFrozenDelta != 0 || entry.DepositDelta != 0 || entry.DepositRefundingDelta != 0 {
			return false, fail(409, "历史余额原付款账单无法核对，请平台核账")
		}
	default:
		return false, fail(409, "该历史支付通道不能自动退款，请平台核账")
	}
	if e.ProviderID != "" {
		entry, err := get[WalletEntry](ctx, q, "wallet_entries", walletKey(e.ProviderID, "order_pending", o.ID), false)
		expected := settlementForOrder(o).ProviderIncomeCents
		if err != nil || entry.UserID != e.ProviderID || entry.Kind != "order_pending" || entry.ReferenceID != o.ID || entry.AmountCents != expected || entry.PendingDelta != expected || entry.AvailableDelta != 0 || entry.RechargeDelta != 0 || entry.WithdrawalFrozenDelta != 0 || entry.DepositDelta != 0 || entry.DepositRefundingDelta != 0 {
			return false, fail(409, "历史托管原待结算账单无法核对，请平台核账")
		}
	}
	return true, nil
}
