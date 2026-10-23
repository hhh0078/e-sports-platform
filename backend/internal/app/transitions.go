// Package app 以订单行锁保护接单、交付、退款和平台介入的状态流转。
package app

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// applyAction 对交易状态机执行纯内存校验与修改，调用方负责持锁提交。
func applyAction(o *Order, u User, action, note string) error {
	if err := requireLiveOrder(*o); err != nil {
		return err
	}
	if o.ActiveAdjustmentID != "" {
		return fail(409, "当前有未完成调价，请先回应、撤回或同步调价资金状态")
	}
	buyer := o.BuyerID == u.ID
	provider := o.ProviderID == u.ID
	note = strings.TrimSpace(note)
	if !length(note, 0, 2000) {
		return fail(400, "说明最多 2000 字")
	}
	pendingRefund := contains([]string{"requested", "escalated", "processing"}, o.RefundStatus)
	switch action {
	case "claim":
		if buyer {
			return fail(400, "不能接自己的订单")
		}
		if o.Status != "open" || o.ProviderID != "" || o.PaymentStatus != "paid" || pendingRefund {
			return fail(409, "订单已被接走或不可接单")
		}
		o.ProviderID = u.ID
		o.ProviderName = u.Nickname
		o.Status = "accepted"
	case "accept":
		if !provider {
			return fail(403, "仅指定打手可以接单")
		}
		if o.Status != "open" || o.PaymentStatus != "paid" || pendingRefund {
			return fail(409, "当前订单不可接单")
		}
		o.Status = "accepted"
	case "start":
		if !provider {
			return fail(403, "只有接单人可以开始服务")
		}
		if o.Status != "accepted" || pendingRefund {
			return fail(409, "当前订单不可开始服务")
		}
		o.Status = "in_progress"
	case "deliver":
		if !provider {
			return fail(403, "只有接单人可以提交交付")
		}
		if o.Status != "in_progress" || pendingRefund {
			return fail(409, "当前订单不可交付")
		}
		if !length(note, 2, 2000) {
			return fail(400, "请填写至少 2 字的交付说明")
		}
		o.Status = "delivered"
		o.DeliveryNote = note
	case "confirm":
		if !buyer {
			return fail(403, "只有买家可以确认完成")
		}
		if o.Status != "delivered" || pendingRefund {
			return fail(409, "订单尚未交付或退款正在处理中")
		}
		o.Status = "completed"
		o.CompletedAt = now()
	case "cancel":
		if !buyer {
			return fail(403, "只有买家可以取消订单")
		}
		if o.Status != "pending_payment" && o.Status != "open" {
			return fail(409, "已接单订单请使用申请退款")
		}
		if pendingRefund {
			return fail(409, "退款正在处理中")
		}
		if o.PaymentStatus == "paid" {
			o.Status = "refunded"
			o.PaymentStatus = "refunded"
			o.RefundStatus = "approved"
			o.RefundReason = "买家在接单前取消订单"
		} else {
			o.Status = "cancelled"
		}
	case "request_refund", "dispute":
		if o.RefundFinal {
			return fail(409, "平台已作最终裁决，不可重复申请退款或介入")
		}
		if !buyer {
			return fail(403, "只有买家可以申请退款")
		}
		if o.PaymentStatus != "paid" || contains([]string{"cancelled", "refunded"}, o.Status) || pendingRefund {
			return fail(409, "当前订单不可申请退款")
		}
		if !length(note, 2, 2000) {
			return fail(400, "请填写至少 2 字的退款原因")
		}
		if o.Status == "completed" {
			t, e := time.Parse(time.RFC3339Nano, o.CompletedAt)
			if e != nil || time.Since(t) > 7*24*time.Hour {
				return fail(409, "已超过完成后 7 天售后期限")
			}
		}
		o.RefundStatus = "requested"
		o.RefundReason = note
		o.RefundReply = ""
		if action == "dispute" {
			o.RefundStatus = "escalated"
			o.DisputeReason = note
		}
	case "approve_refund":
		if !provider {
			return fail(403, "只有接单人可以同意退款")
		}
		if o.RefundStatus != "requested" {
			return fail(409, "没有待处理的退款申请")
		}
		o.RefundStatus = "approved"
		o.PaymentStatus = "refunded"
		o.Status = "refunded"
		o.RefundReply = note
	case "reject_refund":
		if !provider {
			return fail(403, "只有接单人可以拒绝退款")
		}
		if o.RefundStatus != "requested" {
			return fail(409, "没有待处理的退款申请")
		}
		if !length(note, 2, 2000) {
			return fail(400, "请填写拒绝原因")
		}
		o.RefundStatus = "rejected"
		o.RefundReply = note
	case "escalate":
		if o.RefundFinal {
			return fail(409, "平台已作最终裁决，不可再次申请介入")
		}
		if !buyer {
			return fail(403, "只有买家可以申请平台介入")
		}
		if !contains([]string{"requested", "rejected"}, o.RefundStatus) {
			return fail(409, "请先发起退款申请")
		}
		o.RefundStatus = "escalated"
		o.DisputeReason = o.RefundReason
		if note != "" {
			o.DisputeReason = note
		}
	case "withdraw_refund":
		if !buyer {
			return fail(403, "只有买家可以撤回退款")
		}
		if !contains([]string{"requested", "escalated", "rejected"}, o.RefundStatus) {
			return fail(409, "没有可撤回的退款申请")
		}
		o.RefundStatus = "none"
		o.RefundReply = "买家主动撤回退款申请"
	default:
		return fail(400, "操作类型不存在")
	}
	o.UpdatedAt = now()
	return nil
}

// action 对订单加行锁后检查权限并同步记录系统消息。
func (a *App) action(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Action string `json:"action"`
		Note   string `json:"note"`
		ClubID string `json:"club_id"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Action == "claim" || in.Action == "accept" {
		if err = a.requireVerifiedPhone(r, u); err != nil {
			return nil, err
		}
	}
	if err = a.moderateOrderNote(r, u, in.Action, in.Note); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if in.Action == "claim" && in.ClubID != "" {
		if err = lockClubFinance(r.Context(), tx, in.ClubID); err != nil {
			return nil, err
		}
	}
	o, err := lockWalletOrder(r.Context(), tx, r.PathValue("id"), u.ID)
	if err != nil {
		return nil, err
	}
	if err = requireLiveOrder(o); err != nil {
		return nil, err
	}
	before := o
	if in.Action == "claim" || in.Action == "accept" {
		if err = guardDepositRefunding(r.Context(), tx, o.BuyerID, u.ID); err != nil {
			return nil, err
		}
	}
	if in.Action == "cancel" && !a.Config.DevMode && before.PaymentChannel != "balance" && o.Status == "pending_payment" && o.PayOpenID != "" {
		if u.ID != o.BuyerID {
			return nil, fail(403, "只有买家可以取消订单")
		}
		if err = a.closePayment(r.Context(), o); err != nil {
			return nil, err
		}
	}
	if err = applyAction(&o, u, in.Action, in.Note); err != nil {
		return nil, err
	}
	if in.ClubID != "" && in.Action != "claim" {
		return nil, fail(400, "仅领取公开需求时可选择俱乐部")
	}
	if in.Action == "claim" {
		o.ClubID = ""
		if in.ClubID != "" {
			c, e := get[Club](r.Context(), tx, "clubs", in.ClubID, false)
			m, me := get[ClubMember](r.Context(), tx, "members", in.ClubID+":"+u.ID, false)
			if e != nil || c.Status != "active" || !contains(c.GameIDs, o.GameID) || (c.OwnerID != u.ID && (me != nil || m.Status != "active")) {
				return nil, fail(403, "只能代表本人已通过挂靠且支持该游戏的俱乐部接单")
			}
			if err = guardClubDeposit(r.Context(), tx, in.ClubID); err != nil {
				return nil, err
			}
			if err = lockWalletUsers(r.Context(), tx, clubWalletID(in.ClubID)); err != nil {
				return nil, err
			}
			o.ClubID = in.ClubID
		}
	}

	adjustedRefund := before.AdjustmentVersion > 0 && before.PaymentStatus == "paid" && o.PaymentStatus == "refunded"
	if adjustedRefund {
		if err = prepareAdjustedFullRefund(r.Context(), tx, &o, before); err != nil {
			return nil, err
		}
	}
	remoteRefund := !adjustedRefund && before.PaymentStatus == "paid" && o.PaymentStatus == "refunded" && !a.Config.DevMode && before.PaymentChannel != "balance"
	if remoteRefund {
		if err = a.prepareRemoteRefund(&o, before); err != nil {
			return nil, err
		}
	}
	if in.Action == "claim" {
		if err = freezeOrderSettlement(r.Context(), tx, &o); err != nil {
			return nil, err
		}
		if err = claimWalletEscrow(r.Context(), tx, o); err != nil {
			return nil, err
		}
	}
	if o.PaymentStatus == "refunded" && before.PaymentStatus == "paid" {
		if err = refundWalletEscrow(r.Context(), tx, &o); err != nil {
			return nil, err
		}
	}
	if err = put(r.Context(), tx, "orders", o.ID, o); err != nil {
		return nil, err
	}
	labels := map[string]string{"claim": "领取需求", "accept": "接受订单", "start": "开始服务", "deliver": "提交交付", "confirm": "确认完成", "cancel": "取消订单", "request_refund": "申请退款", "approve_refund": "同意退款", "reject_refund": "拒绝退款", "escalate": "申请平台介入", "withdraw_refund": "撤回退款", "dispute": "申请平台介入"}
	text := u.Nickname + "：" + labels[in.Action]
	if strings.TrimSpace(in.Note) != "" {
		text += "。说明：" + strings.TrimSpace(in.Note)
	}
	if err = systemMessage(r.Context(), tx, o, text); err != nil {
		return nil, err
	}
	if err = refreshServiceStats(r.Context(), tx, o.ServiceID); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	if remoteRefund {
		return a.submitRemoteRefund(r.Context(), o, in.Note)
	}
	if adjustedRefund {
		if err = a.processOrderRefunds(r.Context(), o.ID, "full:"+o.ID); err != nil {
			return nil, err
		}
		return get[Order](r.Context(), a.DB, "orders", o.ID, false)
	}
	return o, nil
}

// refreshServiceStats 依据已完成且未退款订单计算服务成交与评分。
func refreshServiceStats(ctx context.Context, q querier, id string) error {
	if id == "" {
		return nil
	}
	s, err := get[Service](ctx, q, "services", id, true)
	if err != nil {
		return err
	}
	if err = q.QueryRow(ctx, "SELECT count(*) FROM orders WHERE data->>'service_id'=$1 AND data->>'status'='completed'", id).Scan(&s.CompletedCount); err != nil {
		return err
	}
	if err = q.QueryRow(ctx, "SELECT COALESCE(avg((r.data->>'rating')::numeric),0) FROM reviews r JOIN orders o ON o.id=r.data->>'order_id' WHERE o.data->>'service_id'=$1 AND o.data->>'status'='completed'", id).Scan(&s.Rating); err != nil {
		return err
	}
	return put(ctx, q, "services", id, s)
}
