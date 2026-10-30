// Package app 按独立补款和部分退款的真实发生日统计交易，调价不能改写初付款日的成交额。
package app

import (
	"context"
	"time"
)

// adjustmentPaymentEvidence 核对每笔不可变原付款，统计不依赖可随净价更新的订单总额。
func adjustmentPaymentEvidence(ctx context.Context, q querier, o Order, p OrderPayment, dev bool) bool {
	if p.OrderID != o.ID || p.BuyerID != o.BuyerID || p.Status != "paid" || p.AmountCents <= 0 || p.RefundedCents < 0 || p.ReservedCents < 0 || p.RefundedCents+p.ReservedCents > p.AmountCents {
		return false
	}
	if p.Kind == "base" {
		if p.AmountCents != o.OriginalAmountCents || p.Channel != o.PaymentChannel || p.OrderNo != o.OrderNo {
			return false
		}
	} else {
		v, e := get[OrderAdjustment](ctx, q, "order_adjustments", p.AdjustmentID, false)
		if e != nil || v.Kind != "increase" || v.AmountCents != p.AmountCents || v.Status != "completed" || v.OrderID != o.ID {
			return false
		}
	}
	if p.Channel == "balance" {
		kind, reference := "order_adjustment_payment", p.ID
		if p.Kind == "base" {
			kind = "order_payment"
			reference = o.ID
		}
		entry, e := get[WalletEntry](ctx, q, "wallet_entries", walletKey(o.BuyerID, kind, reference), false)
		return e == nil && entry.AmountCents == -p.AmountCents && entry.RechargeDelta == -p.RechargeUsed && entry.AvailableDelta == -p.IncomeUsed && p.RechargeUsed >= 0 && p.IncomeUsed >= 0 && p.RechargeUsed+p.IncomeUsed == p.AmountCents
	}
	if p.Channel == "virtual" {
		cons, e := get[virtualConsumption](ctx, q, "virtual_consumptions", virtualConsumptionID(o.ID, p.AdjustmentID), false)
		return e == nil && cons.Status == "paid" && cons.UserID == p.BuyerID && cons.Quote.ServiceCents == p.AmountCents && cons.OrderNo == p.TransactionID && cons.AppID == p.AppID && p.MerchantID == "virtual:"+cons.OfferID
	}
	if p.Channel != "wechat" && !(dev && p.Channel == "dev") {
		return false
	}
	kind, id := "order_payment", p.ID
	if p.Kind == "base" {
		kind = "order"
		id = o.ID
	}
	r, e := get[FundingReceipt](ctx, q, "fund_receipts", walletKey("receipt", kind, id), false)
	return e == nil && r.AmountCents == p.AmountCents && r.TransactionID == p.TransactionID && r.Channel == p.Channel && r.MerchantID == p.MerchantID && r.AppID == p.AppID
}

// addAdjustmentAnalytics 先核验整套原单和退款，再分别累计事件，退款笔数按订单去重。
func (a *App) addAdjustmentAnalytics(ctx context.Context, o Order, esc WalletEscrow, out *AnalyticsResult, indices map[string]int, location *time.Location) (bool, error) {
	if !o.WalletAccounted || o.OriginalAmountCents <= 0 || o.AmountCents != o.PaidTotalCents-o.PriceRefundedCents || esc.OrderID != o.ID || esc.AmountCents != o.AmountCents || esc.BuyerID != o.BuyerID || esc.ProviderID != o.ProviderID || !contains([]string{"holding", "released", "refunded"}, esc.Status) || !contains([]string{"paid", "refunded"}, o.PaymentStatus) {
		return false, nil
	}
	payments, e := list[OrderPayment](ctx, a.DB, "order_payments", "WHERE data->>'order_id'=$1 AND data->>'status'='paid' AND data->>'kind'<>'late_supplement'", o.ID)
	if e != nil {
		return false, e
	}
	refunds, e := list[OrderPartRefund](ctx, a.DB, "order_refunds", "WHERE data->>'order_id'=$1 AND data->>'status'='success' AND COALESCE(data->>'purpose','')<>'late'", o.ID)
	if e != nil {
		return false, e
	}
	var paid int64
	byID := map[string]OrderPayment{}
	refunded := map[string]int64{}
	base := 0
	for _, p := range payments {
		if !adjustmentPaymentEvidence(ctx, a.DB, o, p, a.Config.DevMode) {
			return false, nil
		}
		if _, e = time.Parse(time.RFC3339Nano, p.PaidAt); e != nil {
			return false, nil
		}
		paid += p.AmountCents
		byID[p.ID] = p
		if p.Kind == "base" {
			base++
		}
	}
	if paid != o.PaidTotalCents || base != 1 {
		return false, nil
	}
	for _, r := range refunds {
		p, found := byID[r.PaymentID]
		if !found || r.AmountCents <= 0 || r.AmountCents > p.AmountCents {
			return false, nil
		}
		if _, e = time.Parse(time.RFC3339Nano, r.SuccessAt); e != nil {
			return false, nil
		}
		refunded[p.ID] += r.AmountCents
	}
	for _, p := range payments {
		if refunded[p.ID] != p.RefundedCents {
			return false, nil
		}
	}
	for _, p := range payments {
		stamp, _ := time.Parse(time.RFC3339Nano, p.PaidAt)
		if index, ok := indices[stamp.In(location).Format("2006-01-02")]; ok {
			out.Series[index].PaidCents += p.AmountCents
			out.Summary.GMVCents += p.AmountCents
			if p.Kind == "base" {
				out.Series[index].PaidOrders++
				out.Summary.PaidOrderCount++
			}
		}
	}
	countedDays := map[int]bool{}
	counted := false
	for _, r := range refunds {
		stamp, _ := time.Parse(time.RFC3339Nano, r.SuccessAt)
		if index, ok := indices[stamp.In(location).Format("2006-01-02")]; ok {
			out.Series[index].RefundCents += r.AmountCents
			out.Summary.RefundCents += r.AmountCents
			if !countedDays[index] {
				out.Series[index].RefundedOrders++
				countedDays[index] = true
			}
			if !counted {
				out.Summary.RefundedOrderCount++
				counted = true
			}
		}
	}
	return true, nil
}
