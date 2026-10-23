// Package app 在订单统一锁内处理双方调价确认、过期和证据，资金成功前不修改净价。
package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const orderAdjustmentLifetime = 24 * time.Hour

// baseOrderAmount 返回不随后续加减价改变的初次实付金额。
func baseOrderAmount(o Order) int64 {
	if o.AdjustmentVersion > 0 {
		return o.OriginalAmountCents
	}
	return o.AmountCents
}

// expireOrderAdjustment 只过期尚未出网付款的协商，未知支付和退款不能自动解冻。
func expireOrderAdjustment(ctx context.Context, q querier, o *Order) error {
	if o.ActiveAdjustmentID == "" {
		return nil
	}
	v, e := get[OrderAdjustment](ctx, q, "order_adjustments", o.ActiveAdjustmentID, true)
	if e != nil {
		return e
	}
	expires, e := time.Parse(time.RFC3339Nano, v.ExpiresAt)
	if e != nil {
		return fail(409, "调价有效期异常，请平台核查")
	}
	if !contains([]string{"proposed", "awaiting_payment"}, v.Status) || time.Now().Before(expires) {
		return nil
	}
	v.Status = "expired"
	v.UpdatedAt = now()
	o.ActiveAdjustmentID = ""
	o.UpdatedAt = now()
	if e = put(ctx, q, "order_adjustments", v.ID, v); e != nil {
		return e
	}
	if e = systemMessage(ctx, q, *o, "调价方案超过24小时未完成，已自动失效；原订单金额不变。"); e != nil {
		return e
	}
	return put(ctx, q, "orders", o.ID, *o)
}

// adjustmentEligible 校验履约、售后及托管边界，完成后只在未结算的7天内允许减价。
func adjustmentEligible(o Order, kind string) error {
	if e := requireLiveOrder(o); e != nil {
		return e
	}
	if o.PaymentStatus != "paid" || o.ProviderID == "" || !contains([]string{"open", "accepted", "in_progress", "delivered", "completed"}, o.Status) {
		return fail(409, "订单尚未付款、未确定打手或已关闭，无法调价")
	}
	if contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) || o.RefundFinal || o.RefundStatus == "rejected" {
		return fail(409, "订单正在售后或已有裁决，请先处理原售后")
	}
	if o.Status == "completed" {
		completed, e := time.Parse(time.RFC3339Nano, o.CompletedAt)
		if kind != "decrease" || e != nil || !time.Now().Before(completed.Add(walletHoldDuration)) {
			return fail(409, "订单完成后仅可在7天售后期且未结算时协商减价")
		}
	}
	return nil
}

// ensureOrderPaymentLedger 首次调价时以原始凭证登记初付款，不为缺失来源的历史订单补造资金。
func ensureOrderPaymentLedger(ctx context.Context, q querier, o *Order, dev bool) error {
	e, err := get[WalletEscrow](ctx, q, "wallet_escrows", o.ID, true)
	if err != nil {
		return err
	}
	if e.Status != "holding" || e.AmountCents != o.AmountCents || e.BuyerID != o.BuyerID || e.ProviderID != o.ProviderID || e.FundingVersion != 1 || e.LegacySourceGap || o.FinancialReconciliationRequired {
		return fail(409, "订单已结算或来源待核账，暂不能调价")
	}
	lots, err := fundingLots(ctx, q, "escrow:"+o.ID, "escrow")
	if err != nil {
		return err
	}
	var total int64
	for _, l := range lots {
		total += l.RemainingCents
		if err = verifyFundingLot(ctx, q, l, dev, map[string]bool{}); err != nil {
			return err
		}
	}
	if total != o.AmountCents {
		return fail(409, "订单托管来源与净价不一致，请平台核账")
	}
	if o.AdjustmentVersion > 0 {
		return nil
	}
	if _, parseErr := time.Parse(time.RFC3339Nano, o.PaidAt); parseErr != nil {
		return fail(409, "原付款发生时间待核账，暂不能自动调价")
	}
	transaction := o.PaymentTransactionID
	if o.PaymentChannel == "dev" {
		transaction = "DEVORDER" + o.ID
	}
	p := OrderPayment{ID: walletKey("base-order-payment", o.ID), OrderID: o.ID, BuyerID: o.BuyerID, Kind: "base", Channel: o.PaymentChannel, AmountCents: o.AmountCents, Status: "paid", OrderNo: o.OrderNo, TransactionID: transaction, MerchantID: o.PaymentMerchantID, AppID: o.PaymentAppID, PayOpenID: o.PayOpenID, RechargeUsed: e.RechargeUsed, IncomeUsed: e.IncomeUsed, CreatedAt: o.PaidAt, PaidAt: o.PaidAt, UpdatedAt: now()}
	if p.Channel != "balance" && transaction == "" {
		return fail(409, "订单原付款流水缺失")
	}
	if err = put(ctx, q, "order_payments", p.ID, p); err != nil {
		return err
	}
	o.AdjustmentVersion = 1
	o.OriginalAmountCents = o.AmountCents
	o.PaidTotalCents = o.AmountCents
	return nil
}

// orderAdjustments 仅向本订单四方参与者展示协商，读取时处理未付款方案过期。
func (a *App) orderAdjustments(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	if _, e = a.authorizedOrder(r, u, r.PathValue("id"), false); e != nil {
		return nil, e
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
	items, e := list[OrderAdjustment](r.Context(), tx, "order_adjustments", "WHERE data->>'order_id'=$1 ORDER BY data->>'created_at',id", o.ID)
	if e != nil {
		return nil, e
	}
	return items, tx.Commit(r.Context())
}

// proposeOrderAdjustment 把正整数差额及说明绑定到申请人幂等号，增加费用只允许本单打手提出。
func (a *App) proposeOrderAdjustment(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	var in struct {
		Kind        string `json:"kind"`
		AmountCents int64  `json:"amount_cents"`
		Reason      string `json:"reason"`
		RequestID   string `json:"request_id"`
	}
	if e = decode(r, &in); e != nil {
		return nil, e
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !contains([]string{"increase", "decrease"}, in.Kind) || in.AmountCents < 1 || in.AmountCents > 10000000 || !length(in.Reason, 2, 1000) {
		return nil, fail(400, "请选择加价或减价，填写有效差额及2至1000字原因")
	}
	if e = walletRequestID(in.RequestID); e != nil {
		return nil, e
	}
	o, e := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if e != nil {
		return nil, e
	}
	if u.ID != o.ProviderID && (in.Kind == "increase" || u.ID != o.BuyerID) {
		return nil, fail(403, "加价仅打手可提出，减价仅交易双方可提出")
	}
	prior, priorErr := get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", walletKey("order-adjustment", u.ID, in.RequestID), false)
	if priorErr == nil {
		if prior.OrderID != o.ID || prior.Kind != in.Kind || prior.AmountCents != in.AmountCents || prior.Reason != in.Reason {
			return nil, fail(409, "该请求号已经用于不同调价方案")
		}
		return prior, nil
	}
	if !missing(priorErr) {
		return nil, priorErr
	}
	if e = adjustmentEligible(o, in.Kind); e != nil {
		return nil, e
	}
	if e = a.moderateText(r, u, "订单调价说明", in.Reason); e != nil {
		return nil, e
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(r.Context())
	o, e = lockWalletOrder(r.Context(), tx, o.ID, u.ID)
	if e != nil {
		return nil, e
	}
	id := walletKey("order-adjustment", u.ID, in.RequestID)
	old, e := get[OrderAdjustment](r.Context(), tx, "order_adjustments", id, false)
	if e == nil {
		if old.OrderID != o.ID || old.Kind != in.Kind || old.AmountCents != in.AmountCents || old.Reason != in.Reason {
			return nil, fail(409, "该请求号已经用于不同调价方案")
		}
		return old, tx.Commit(r.Context())
	}
	if !missing(e) {
		return nil, e
	}
	if e = adjustmentEligible(o, in.Kind); e != nil {
		return nil, e
	}
	if o.ActiveAdjustmentID != "" {
		return nil, fail(409, "请先完成、拒绝或撤回当前调价方案")
	}
	if in.Kind == "increase" && o.AmountCents > 10000000-in.AmountCents || in.Kind == "decrease" && in.AmountCents >= o.AmountCents {
		return nil, fail(400, "调整后金额须大于0且不超过100000元，全额退还请申请退款")
	}
	if e = ensureOrderPaymentLedger(r.Context(), tx, &o, a.Config.DevMode); e != nil {
		return nil, e
	}
	v := OrderAdjustment{ID: id, OrderID: o.ID, Kind: in.Kind, AmountCents: in.AmountCents, Reason: in.Reason, RequestID: in.RequestID, ProposerID: u.ID, ProposerName: u.Nickname, Status: "proposed", BuyerAgreed: u.ID == o.BuyerID, ProviderAgreed: u.ID == o.ProviderID, CreatedAt: now(), UpdatedAt: now(), ExpiresAt: time.Now().Add(orderAdjustmentLifetime).UTC().Format(time.RFC3339Nano)}
	o.ActiveAdjustmentID = v.ID
	o.UpdatedAt = now()
	if e = put(r.Context(), tx, "order_adjustments", v.ID, v); e != nil {
		return nil, e
	}
	if e = put(r.Context(), tx, "orders", o.ID, o); e != nil {
		return nil, e
	}
	label := "增加"
	if in.Kind == "decrease" {
		label = "减少"
	}
	if e = systemMessage(r.Context(), tx, o, fmt.Sprintf("%s 提议%s费用 ¥%.2f。原因：%s。需另一方确认，原价暂不改变。", u.Nickname, label, float64(v.AmountCents)/100, v.Reason)); e != nil {
		return nil, e
	}
	return v, tx.Commit(r.Context())
}

// respondOrderAdjustment 在同一订单锁内保存对方同意，减价先预留各原单再于事务外退款。
func (a *App) respondOrderAdjustment(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	var in struct {
		Action string `json:"action"`
	}
	if e = decode(r, &in); e != nil {
		return nil, e
	}
	if !contains([]string{"accept", "reject", "withdraw"}, in.Action) {
		return nil, fail(400, "调价回应无效")
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
	if u.ID != o.BuyerID && u.ID != o.ProviderID {
		return nil, fail(403, "仅买家和本单打手可确认调价")
	}
	v, e := get[OrderAdjustment](r.Context(), tx, "order_adjustments", r.PathValue("adjustment_id"), true)
	if e != nil {
		return nil, e
	}
	if v.OrderID != o.ID {
		return nil, fail(404, "调价方案不存在")
	}
	if contains([]string{"completed", "rejected", "withdrawn", "expired"}, v.Status) {
		return v, tx.Commit(r.Context())
	}
	if in.Action == "withdraw" && u.ID != v.ProposerID || in.Action != "withdraw" && u.ID == v.ProposerID {
		return nil, fail(403, "请由对方回应方案，发起人可撤回")
	}
	if in.Action == "accept" && v.BuyerAgreed && v.ProviderAgreed {
		return v, tx.Commit(r.Context())
	}
	if !contains([]string{"proposed", "awaiting_payment"}, v.Status) {
		return nil, fail(409, "资金正在处理，请同步原调价单状态")
	}
	if e = adjustmentEligible(o, v.Kind); e != nil {
		return nil, e
	}
	if in.Action == "accept" {
		if u.ID == o.BuyerID {
			v.BuyerAgreed = true
		} else {
			v.ProviderAgreed = true
		}
		if !v.BuyerAgreed || !v.ProviderAgreed {
			return nil, fail(409, "需要交易双方同意")
		}
		v.Status = "awaiting_payment"
		if v.Kind == "decrease" {
			v.Status = "refunding"
			if e = planOrderRefunds(r.Context(), tx, o, v.ID, v.ID, v.AmountCents, v.Reason); e != nil {
				return nil, e
			}
		}
	} else {
		v.Status = "rejected"
		if in.Action == "withdraw" {
			v.Status = "withdrawn"
		}
		o.ActiveAdjustmentID = ""
	}
	v.UpdatedAt = now()
	o.UpdatedAt = now()
	if e = put(r.Context(), tx, "order_adjustments", v.ID, v); e != nil {
		return nil, e
	}
	if e = put(r.Context(), tx, "orders", o.ID, o); e != nil {
		return nil, e
	}
	labels := map[string]string{"accept": "同意", "reject": "拒绝", "withdraw": "撤回"}
	if e = systemMessage(r.Context(), tx, o, u.Nickname+" "+labels[in.Action]+"调价方案。减价以原支付渠道退款结果为准，加价以补款核验成功为准。"); e != nil {
		return nil, e
	}
	if e = tx.Commit(r.Context()); e != nil {
		return nil, e
	}
	if v.Status == "refunding" {
		if e = a.processOrderRefunds(r.Context(), o.ID, v.ID); e != nil {
			return nil, e
		}
		return get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", v.ID, false)
	}
	return v, nil
}
