// Package app 将订单减价或最终全退拆到各原支付单，成功分笔只释放其精确来源容量。
package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// paymentOwnsEscrowLot 以不可变收款根或余额支付分配凭证区分每笔原单的托管批次。
func paymentOwnsEscrowLot(ctx context.Context, q querier, p OrderPayment, l FundingLot) (bool, error) {
	if p.Channel == "virtual" {
		return l.MovementID == walletKey("fund-move", "virtual-pay:"+virtualConsumptionID(p.OrderID, p.AdjustmentID)), nil
	}
	if p.Channel == "balance" {
		prefix := "adjustment-pay:" + p.ID + ":"
		if p.Kind == "base" {
			prefix = "order-pay:" + p.OrderID + ":"
		}
		return l.MovementID == walletKey("fund-move", prefix+"recharge") || l.MovementID == walletKey("fund-move", prefix+"available"), nil
	}
	r, e := get[FundingReceipt](ctx, q, "fund_receipts", l.ReceiptID, false)
	if e != nil {
		return false, e
	}
	kind, id := "order_payment", p.ID
	if p.Kind == "base" {
		kind = "order"
		id = p.OrderID
	}
	return r.Kind == kind && r.PaymentID == id && r.TransactionID == p.TransactionID && r.AmountCents == p.AmountCents && r.Channel == p.Channel, nil
}

// planOrderRefunds 在统一资金锁中按最新付款优先预留金额及来源，全退复用同一组号。
func planOrderRefunds(ctx context.Context, q querier, o Order, adjustmentID, group string, amount int64, reason string) error {
	old, e := list[OrderPartRefund](ctx, q, "order_refunds", "WHERE data->>'order_id'=$1 AND data->>'group_id'=$2", o.ID, group)
	if e != nil {
		return e
	}
	if len(old) > 0 {
		return nil
	}
	payments, e := list[OrderPayment](ctx, q, "order_payments", "WHERE data->>'order_id'=$1 AND data->>'status'='paid' AND data->>'kind'<>'late_supplement' ORDER BY (data->>'paid_at')::timestamptz DESC,id DESC FOR UPDATE", o.ID)
	if e != nil {
		return e
	}
	lots, e := fundingLots(ctx, q, "escrow:"+o.ID, "escrow")
	if e != nil {
		return e
	}
	remaining := amount
	for _, p := range payments {
		available := p.AmountCents - p.RefundedCents - p.ReservedCents
		if available <= 0 {
			continue
		}
		take := available
		if take > remaining {
			take = remaining
		}
		if take <= 0 {
			break
		}
		if p.Channel == "virtual" && take%100 != 0 {
			return fail(409, "官方1:1金币小数退币精度未核实，当前退款金额须为整金币，未舍入退款额")
		}
		part := OrderPartRefund{ID: walletKey("order-part-refund", group, p.ID), OrderID: o.ID, AdjustmentID: adjustmentID, GroupID: group, PaymentID: p.ID, RefundNo: "OR" + strings.ToUpper(walletKey(group, p.ID)[:60]), AmountCents: take, Status: "queued", Reason: reason, Sources: []FundingPart{}, CreatedAt: now(), UpdatedAt: now()}
		need := take
		for _, l := range lots {
			owns, e := paymentOwnsEscrowLot(ctx, q, p, l)
			if e != nil {
				return e
			}
			if !owns {
				continue
			}
			piece := l.RemainingCents
			if piece > need {
				piece = need
			}
			if piece > 0 {
				part.Sources = append(part.Sources, FundingPart{l.ID, piece})
				need -= piece
			}
			if need == 0 {
				break
			}
		}
		if need != 0 {
			return fail(409, "原付款剩余来源与可退容量不一致，请平台核账")
		}
		p.ReservedCents += take
		p.UpdatedAt = now()
		if e = put(ctx, q, "order_payments", p.ID, p); e != nil {
			return e
		}
		if e = put(ctx, q, "order_refunds", part.ID, part); e != nil {
			return e
		}
		remaining -= take
		if remaining == 0 {
			break
		}
	}
	if remaining != 0 {
		return fail(409, "订单剩余实付不足或退款已被预留")
	}
	return nil
}

// consumeOrderRefundSource 逐分消费已预留批次，余额恢复原资金分区及原收入订单归属。
func consumeOrderRefundSource(ctx context.Context, q querier, o Order, p OrderPayment, r OrderPartRefund) (int64, int64, error) {
	var recharge, income, total int64
	for _, part := range r.Sources {
		l, e := get[FundingLot](ctx, q, "fund_lots", part.LotID, true)
		if e != nil {
			return 0, 0, e
		}
		owns, e := paymentOwnsEscrowLot(ctx, q, p, l)
		if e != nil {
			return 0, 0, e
		}
		account, bucket := "escrow:"+o.ID, "escrow"
		if r.Purpose == "late" {
			account = "overpayment:" + p.ID
			bucket = "overpayment"
		}
		if !owns || l.AccountID != account || l.Bucket != bucket || part.AmountCents <= 0 || part.AmountCents > l.RemainingCents {
			return 0, 0, fail(409, "退款来源归属或剩余容量异常")
		}
		mid := walletKey("fund-move", "order-part-refund:"+r.ID+":"+l.ID)
		if _, e = get[FundingMovement](ctx, q, "fund_movements", mid, false); !missing(e) {
			if e == nil {
				return 0, 0, fail(409, "退款来源分录已存在但退款状态未完成，请核账")
			}
			return 0, 0, e
		}
		m := FundingMovement{ID: mid, Inputs: []FundingPart{{l.ID, part.AmountCents}}, Outputs: []FundingPart{}, CreatedAt: now()}
		if p.Channel == "balance" || p.Channel == "virtual" {
			if p.Channel == "virtual" {
				if l.ReturnBucket != "virtual_recharge" {
					return 0, 0, fail(409, "金币退款来源分区不匹配")
				}
			} else {
				if l.ReturnBucket == "recharge" {
					recharge += part.AmountCents
				} else if l.ReturnBucket == "available" {
					income += part.AmountCents
				} else {
					return 0, 0, fail(409, "余额退款缺少原始资金分区")
				}
			}
			parent, e := get[FundingLot](ctx, q, "fund_lots", l.ParentID, false)
			if e != nil {
				return 0, 0, e
			}
			out := FundingLot{ID: walletKey("fund-output", mid, "0"), AccountID: o.BuyerID, Bucket: l.ReturnBucket, ReceiptID: l.ReceiptID, ParentID: l.ID, MovementID: mid, OrderID: parent.OrderID, AmountCents: part.AmountCents, RemainingCents: part.AmountCents, CreatedAt: m.CreatedAt}
			if e = put(ctx, q, "fund_lots", out.ID, out); e != nil {
				return 0, 0, e
			}
			m.Outputs = append(m.Outputs, FundingPart{out.ID, part.AmountCents})
		}
		l.RemainingCents -= part.AmountCents
		if e = put(ctx, q, "fund_lots", l.ID, l); e != nil {
			return 0, 0, e
		}
		if e = put(ctx, q, "fund_movements", mid, m); e != nil {
			return 0, 0, e
		}
		total += part.AmountCents
	}
	if total != r.AmountCents {
		return 0, 0, fail(409, "退款来源分配金额不守恒")
	}
	return recharge, income, nil
}

// processOrderRefunds 在事务外执行原单退款或查询，未知结果不会生成新单或解除冻结。
func (a *App) processOrderRefunds(ctx context.Context, orderID, group string) error {
	parts, e := list[OrderPartRefund](ctx, a.DB, "order_refunds", "WHERE data->>'order_id'=$1 AND data->>'group_id'=$2 ORDER BY id", orderID, group)
	if e != nil {
		return e
	}
	for _, snapshot := range parts {
		if snapshot.Status == "success" {
			continue
		}
		tx, e := a.DB.Begin(ctx)
		if e != nil {
			return e
		}
		_, e = lockWalletOrder(ctx, tx, orderID, "")
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		part, e := get[OrderPartRefund](ctx, tx, "order_refunds", snapshot.ID, true)
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		if part.Status == "success" {
			tx.Rollback(ctx)
			continue
		}
		first := part.Status == "queued"
		part.Status = "processing"
		part.UpdatedAt = now()
		e = put(ctx, tx, "order_refunds", part.ID, part)
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		p, e := get[OrderPayment](ctx, tx, "order_payments", part.PaymentID, false)
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
		if p.Channel == "virtual" {
			if e = a.processVirtualPartRefund(ctx, p, part); e != nil {
				return e
			}
			continue
		}
		if p.Channel == "balance" || a.Config.DevMode && p.Channel == "dev" {
			v := wxRefund{OrderNo: p.OrderNo, OutRefundNo: part.RefundNo, TransactionID: p.TransactionID, Status: "SUCCESS"}
			v.Amount.Total = p.AmountCents
			v.Amount.Refund = part.AmountCents
			v.Amount.Currency = "CNY"
			if e = a.applyOrderPartRefund(ctx, wxPayConfig{}, v, true); e != nil {
				return e
			}
			continue
		}
		c, e := a.loadWXPayForApp(p.AppID)
		if e != nil {
			return e
		}
		if c.MerchantID != p.MerchantID || c.AppID != p.AppID {
			return fail(409, "退款原商户与当前配置不一致，请恢复原商户配置核账")
		}
		var result wxRefund
		if !first {
			e = a.wxRequest(ctx, c, "GET", "/v3/refund/domestic/refunds/"+url.PathEscape(part.RefundNo), nil, &result)
			if e != nil {
				var apiErr *wxAPIError
				if !errors.As(e, &apiErr) || !contains([]string{"RESOURCE_NOT_EXISTS", "NOT_FOUND"}, apiErr.Code) {
					return e
				}
				first = true
			}
		}
		if first {
			reason := []rune(part.Reason)
			if len(reason) > 26 {
				reason = reason[:26]
			}
			payload := map[string]any{"transaction_id": p.TransactionID, "out_refund_no": part.RefundNo, "reason": string(reason), "notify_url": c.RefundNotifyURL, "amount": map[string]any{"refund": part.AmountCents, "total": p.AmountCents, "currency": "CNY"}}
			if e = a.wxRequest(ctx, c, "POST", "/v3/refund/domestic/refunds", payload, &result); e != nil {
				return e
			}
		}
		if result.OrderNo == "" {
			result.OrderNo = p.OrderNo
		}
		if result.MerchantID == "" {
			result.MerchantID = c.MerchantID
		}
		if e = a.applyOrderPartRefund(ctx, c, result, false); e != nil {
			return e
		}
	}
	return nil
}

// applyOrderPartRefund 只认与预留原交易一致的成功退款，单次成功不能提前关闭整组交易。
func (a *App) applyOrderPartRefund(ctx context.Context, c wxPayConfig, v wxRefund, trustedLocal bool) error {
	var id string
	if e := a.DB.QueryRow(ctx, "SELECT id FROM order_refunds WHERE data->>'refund_no'=$1", v.OutRefundNo).Scan(&id); e != nil {
		return fail(404, "订单退款分笔不存在")
	}
	snapshot, e := get[OrderPartRefund](ctx, a.DB, "order_refunds", id, false)
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
	r, e := get[OrderPartRefund](ctx, tx, "order_refunds", id, true)
	if e != nil {
		return e
	}
	p, e := get[OrderPayment](ctx, tx, "order_payments", r.PaymentID, true)
	if e != nil {
		return e
	}
	if !trustedLocal {
		c, e = a.loadWXPayForApp(p.AppID)
		if e != nil {
			return e
		}
	}
	if (!trustedLocal && (c.MerchantID != p.MerchantID || c.AppID != p.AppID || v.MerchantID != p.MerchantID)) || trustedLocal && p.Channel != "balance" && p.Channel != "virtual" && !(a.Config.DevMode && p.Channel == "dev") || v.OrderNo != p.OrderNo || v.TransactionID != p.TransactionID || v.Amount.Total != p.AmountCents || v.Amount.Refund != r.AmountCents || v.Amount.Currency != "CNY" {
		return fail(400, "退款原支付、商户或部分金额不匹配")
	}
	status := v.RefundStatus
	if status == "" {
		status = v.Status
	}
	if r.Status == "success" {
		return nil
	}
	if status != "SUCCESS" {
		if !contains([]string{"PROCESSING", "ABNORMAL", "CLOSED"}, status) {
			return fail(400, "退款状态无效")
		}
		if status != "PROCESSING" && r.AdjustmentID != "" {
			adjustment, readErr := get[OrderAdjustment](ctx, tx, "order_adjustments", r.AdjustmentID, true)
			if readErr != nil {
				return readErr
			}
			reason := "微信退款异常（" + status + "），原退款号继续冻结，请联系平台核查"
			if adjustment.FailureReason != reason {
				adjustment.FailureReason = reason
				adjustment.UpdatedAt = now()
				if e = put(ctx, tx, "order_adjustments", adjustment.ID, adjustment); e != nil {
					return e
				}
				if e = systemMessage(ctx, tx, o, reason); e != nil {
					return e
				}
			}
			return tx.Commit(ctx)
		}
		return nil
	}
	if p.Status != "paid" || p.ReservedCents < r.AmountCents || p.RefundedCents+r.AmountCents > p.AmountCents {
		return fail(409, "退款容量或原付款状态异常")
	}
	recharge, income, e := consumeOrderRefundSource(ctx, tx, o, p, r)
	if e != nil {
		return e
	}
	if p.Channel == "balance" {
		wallet, e := lockedWallet(ctx, tx, o.BuyerID)
		if e != nil {
			return e
		}
		if e = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "order_part_balance_refund", ReferenceID: r.ID, AmountCents: r.AmountCents, RechargeDelta: recharge, AvailableDelta: income, Note: "订单调价或售后退款，恢复原支付余额分区"}); e != nil {
			return e
		}
	}
	if p.Channel == "virtual" {
		if e = applyVirtualRefundBalance(ctx, tx, p, r); e != nil {
			return e
		}
	}
	p.ReservedCents -= r.AmountCents
	p.RefundedCents += r.AmountCents
	p.UpdatedAt = now()
	r.Status = "success"
	r.SuccessAt = now()
	if v.SuccessTime != "" {
		stamp, parseErr := time.Parse(time.RFC3339Nano, v.SuccessTime)
		if parseErr != nil {
			return fail(400, "退款成功时间无效")
		}
		r.SuccessAt = stamp.UTC().Format(time.RFC3339Nano)
	}
	r.UpdatedAt = now()
	if e = put(ctx, tx, "order_payments", p.ID, p); e != nil {
		return e
	}
	if e = put(ctx, tx, "order_refunds", r.ID, r); e != nil {
		return e
	}
	parts, e := list[OrderPartRefund](ctx, tx, "order_refunds", "WHERE data->>'order_id'=$1 AND data->>'group_id'=$2", o.ID, r.GroupID)
	if e != nil {
		return e
	}
	done := true
	for _, part := range parts {
		done = done && part.Status == "success"
	}
	if done {
		if r.Purpose == "late" {
			if e = finishLateOrderRefund(ctx, tx, o, r); e != nil {
				return e
			}
		} else if r.AdjustmentID != "" {
			adjustment, e := get[OrderAdjustment](ctx, tx, "order_adjustments", r.AdjustmentID, true)
			if e != nil {
				return e
			}
			if o.ActiveAdjustmentID != adjustment.ID || adjustment.Status != "refunding" {
				return fail(409, "减价退款与活动方案不一致")
			}
			if e = adjustOrderNet(ctx, tx, &o, adjustment.ID, -adjustment.AmountCents); e != nil {
				return e
			}
			o.PriceRefundedCents += adjustment.AmountCents
			o.ActiveAdjustmentID = ""
			adjustment.Status = "completed"
			adjustment.FailureReason = ""
			adjustment.CompletedAt = r.SuccessAt
			adjustment.UpdatedAt = now()
			if e = put(ctx, tx, "order_adjustments", adjustment.ID, adjustment); e != nil {
				return e
			}
			if e = systemMessage(ctx, tx, o, fmt.Sprintf("减价 ¥%.2f 已按原支付渠道退还，订单当前净价 ¥%.2f。", float64(adjustment.AmountCents)/100, float64(o.AmountCents)/100)); e != nil {
				return e
			}
		} else {
			if e = finishAdjustedFullRefund(ctx, tx, &o, r.SuccessAt); e != nil {
				return e
			}
		}
		if e = put(ctx, tx, "orders", o.ID, o); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// prepareAdjustedFullRefund 保留履约状态并冻结所有剩余实付，后续全退覆盖每笔补款。
func prepareAdjustedFullRefund(ctx context.Context, q querier, o *Order, before Order) error {
	if before.ActiveAdjustmentID != "" {
		return fail(409, "请先完成当前调价或原付款资金查询")
	}
	if e := planOrderRefunds(ctx, q, before, "", "full:"+o.ID, before.AmountCents, "订单售后全额退还剩余实付"); e != nil {
		return e
	}
	o.Status = before.Status
	o.PaymentStatus = "paid"
	o.RefundStatus = "processing"
	o.RefundID = "ADJUSTED_FULL_" + o.ID
	o.RefundReply = "按各笔原支付逐一退款，全部成功后关闭订单"
	o.UpdatedAt = now()
	return nil
}

// finishAdjustedFullRefund 所有剩余实付已退款后反冲净收入，不修改原价或已成功的减价事实。
func finishAdjustedFullRefund(ctx context.Context, q querier, o *Order, stamp string) error {
	escrow, e := get[WalletEscrow](ctx, q, "wallet_escrows", o.ID, true)
	if e != nil {
		return e
	}
	if escrow.Status == "refunded" {
		return nil
	}
	if escrow.Status != "holding" {
		return fail(409, "资金已结算，不能自动退款")
	}
	s := settlementForOrder(*o)
	for _, part := range []struct {
		account string
		amount  int64
	}{{o.ProviderID, s.ProviderIncomeCents}, {clubWalletID(o.ClubID), s.ClubFeeCents}} {
		if part.amount == 0 {
			continue
		}
		wallet, e := lockedWallet(ctx, q, part.account)
		if e != nil {
			return e
		}
		if e = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "order_adjusted_full_refund", ReferenceID: o.ID, AmountCents: -part.amount, PendingDelta: -part.amount, Note: "各笔原支付全额退还完成，撤回剩余待结算款"}); e != nil {
			return e
		}
	}
	lots, e := fundingLots(ctx, q, "escrow:"+o.ID, "escrow")
	if e != nil {
		return e
	}
	if len(lots) != 0 {
		return fail(409, "全退后仍有未匹配托管来源，请平台核账")
	}
	escrow.Status = "refunded"
	o.Status = "refunded"
	o.PaymentStatus = "refunded"
	o.RefundStatus = "approved"
	o.RefundedAt = stamp
	o.UpdatedAt = now()
	if e = put(ctx, q, "wallet_escrows", o.ID, escrow); e != nil {
		return e
	}
	if e = refreshServiceStats(ctx, q, o.ServiceID); e != nil {
		return e
	}
	return systemMessage(ctx, q, *o, "订单全部剩余实付已按各原支付渠道退还，包含已成功补款；交易关闭。")
}
