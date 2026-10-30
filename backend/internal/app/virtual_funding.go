// Package app 在消费和真实提现前刷新虚拟现金来源，已退款或未知来源不得继续转出。
package app

import "context"

// freezeVirtualSource 对原已付现金单的倒退或无法确认状态保留永久核账标记，不凭未知状态伪造现金退款。
func (a *App) freezeVirtualSource(ctx context.Context, p WalletTopup) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = lockWalletUsers(ctx, tx, p.UserID); e != nil {
		return e
	}
	proof, e := get[virtualTransaction](ctx, tx, "virtual_transactions", p.ID, true)
	if e != nil {
		return e
	}
	proof.ReconciliationRequired = true
	proof.CheckedAt = now()
	if e = put(ctx, tx, "virtual_transactions", p.ID, proof); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	return fail(409, "原已付虚拟现金单状态异常，相关来源已冻结待平台核账")
}

// refreshVirtualFunding 在数据库事务外逐笔向官方查证当前资金批次的现金根，失败保持冻结而不凭旧成功快照出款。
func (a *App) refreshVirtualFunding(ctx context.Context, accountID, bucket string) error {
	items, e := list[FundingReceipt](ctx, a.DB, "fund_receipts", `WHERE data->>'channel'='virtual' AND id IN (SELECT data->>'receipt_id' FROM fund_lots WHERE data->>'account_id'=$1 AND data->>'bucket'=$2 AND (data->>'remaining_cents')::bigint>0) ORDER BY id`, accountID, bucket)
	if e != nil {
		return e
	}
	for _, receipt := range items {
		p, e := get[WalletTopup](ctx, a.DB, "wallet_topups", receipt.PaymentID, false)
		if e != nil {
			return e
		}
		if p.Channel != "virtual" || p.Status != "paid" || p.RefundedCents > 0 {
			return fail(409, "原虚拟现金充值已退款或状态异常，相关资金待核账")
		}
		if e = a.syncVirtualTopup(ctx, p); e != nil {
			return e
		}
		proof, e := get[virtualTransaction](ctx, a.DB, "virtual_transactions", p.ID, false)
		if e != nil {
			return e
		}
		if proof.ReconciliationRequired || proof.CashRefundCents > 0 {
			return fail(409, "原虚拟现金充值存在退款或争议，相关资金已冻结")
		}
	}
	return nil
}

// verifyVirtualConsumptionLot 验证金币从用户账户流入服务托管的不可变分配确实对应已完成的官方扣币。
func verifyVirtualConsumptionLot(ctx context.Context, q querier, l FundingLot) error {
	if l.ReturnBucket != "virtual_recharge" || l.Bucket != "escrow" {
		return nil
	}
	items, e := list[virtualConsumption](ctx, q, "virtual_consumptions", "WHERE data->>'order_id'=$1 AND data->>'status'='paid'", l.OrderID)
	if e != nil {
		return e
	}
	for _, p := range items {
		if l.MovementID == walletKey("fund-move", "virtual-pay:"+p.ID) && p.Quote.ServiceCents >= l.AmountCents && p.Quote.TotalCents == p.Quote.ServiceCents+p.Quote.FeeCents {
			return nil
		}
	}
	return fail(409, "服务收入缺少已核验官方金币消费来源，禁止提现")
}
