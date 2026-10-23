// Package app 按原押金充值来源分笔退还，申请即移除有效押金，外部结果未知时持续冻结并可按原退款号补偿。
package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// walletDepositRefunds 只列出本人押金退还申请及分笔处理状态。
func (a *App) walletDepositRefunds(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	return list[WalletDepositRefund](r.Context(), a.DB, "wallet_deposit_refunds", "WHERE data->>'user_id'=$1 ORDER BY (data->>'created_at')::timestamptz DESC,id DESC LIMIT $2 OFFSET $3", u.ID, limit, offset)
}

// guardDepositOrders 在用户事务锁下检查买卖双方关联交易，待付款、履约及售后期间均不得退押金。
func guardDepositOrders(ctx context.Context, q querier, userID string) error {
	orders, err := list[Order](ctx, q, "orders", "WHERE (data->>'buyer_id'=$1 OR data->>'provider_id'=$1) AND id NOT LIKE 'demo_%'", userID)
	if err != nil {
		return err
	}
	for _, o := range orders {
		blocked := o.ActiveAdjustmentID != "" || !contains([]string{"cancelled", "refunded", "completed"}, o.Status) || contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) || o.RefundStatus == "rejected" && !o.RefundFinal
		if o.Status == "completed" {
			completed, e := time.Parse(time.RFC3339Nano, o.CompletedAt)
			blocked = blocked || e != nil || time.Now().Before(completed.Add(walletHoldDuration))
		}
		if blocked {
			return fail(409, "存在待付款、未完成交易或售后中的订单，暂不能退还平台押金")
		}
	}
	return nil
}

// createWalletDepositRefund 在锁内核对相关交易并按充值先后分配原路退款，绝不将押金转成可消费余额。
func (a *App) createWalletDepositRefund(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		AmountCents *int64 `json:"amount_cents"`
		Scope       string `json:"scope"`
		ClubID      string `json:"club_id"`
		RequestID   string `json:"request_id"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if err = walletRequestID(in.RequestID); err != nil {
		return nil, err
	}
	// 原路退款可退到最后一分；缴纳押金的1元门槛不应限制退款尾款。
	minimum := int64(1)
	if in.AmountCents != nil && (*in.AmountCents < minimum || *in.AmountCents > 10000000) {
		return nil, fail(400, "单次退押金金额须在0.01至100000元范围内")
	}
	if !a.Config.DevMode {
		if _, err = a.loadWXPay(); err != nil {
			return nil, err
		}
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	purpose := "deposit"
	if in.Scope == "club_platform" || in.Scope == "member_club" {
		purpose = in.Scope
	} else if in.Scope != "" && in.Scope != "personal" {
		return nil, fail(400, "押金退还类型无效")
	}
	accountID, err := a.depositRefundAccount(ctx, tx, u, purpose, in.ClubID)
	if err != nil {
		return nil, err
	}
	if err = lockWalletUsers(ctx, tx, u.ID, accountID); err != nil {
		return nil, err
	}
	id := walletKey("deposit_refund", u.ID, in.RequestID)
	v, err := get[WalletDepositRefund](ctx, tx, "wallet_deposit_refunds", id, true)
	if err == nil {
		if v.ClubID != in.ClubID || v.Scope != in.Scope || in.AmountCents != nil && v.AmountCents != *in.AmountCents {
			return nil, fail(409, "同一request_id不能修改退押金金额")
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		if v.Status != "success" {
			if err = a.processDepositRefund(ctx, v); err != nil {
				return nil, err
			}
		}
		return get[WalletDepositRefund](ctx, a.DB, "wallet_deposit_refunds", id, false)
	}
	if !missing(err) {
		return nil, err
	}
	if purpose == "club_platform" {
		err = guardClubDepositRefund(ctx, tx, in.ClubID)
	} else if purpose == "member_club" {
		err = guardMemberDepositRefund(ctx, tx, in.ClubID, u.ID)
	} else {
		err = guardDepositOrders(ctx, tx, u.ID)
	}
	if err != nil {
		return nil, err
	}
	wallet, err := lockedWallet(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	amount := wallet.DepositCents
	if in.AmountCents != nil {
		amount = *in.AmountCents
	}
	if amount < minimum || amount > wallet.DepositCents {
		return nil, fail(409, "有效押金不足最低退款金额或申请金额超过可退余额")
	}
	topups, err := list[WalletTopup](ctx, tx, "wallet_topups", "WHERE data->>'user_id'=$1 AND data->>'purpose'=$2 AND COALESCE(data->>'club_id','')=$3 AND data->>'status'='paid' ORDER BY (data->>'created_at')::timestamptz,id", u.ID, purpose, in.ClubID)
	if err != nil {
		return nil, err
	}
	v = WalletDepositRefund{AccountID: accountID, Scope: in.Scope, ClubID: in.ClubID, ID: id, UserID: u.ID, RequestID: in.RequestID, AmountCents: amount, Status: "processing", Parts: []DepositRefundPart{}, CreatedAt: now(), UpdatedAt: now()}
	remaining := amount
	for _, p := range topups {
		available := p.AmountCents - p.RefundedCents - p.ReservedCents
		if available <= 0 {
			continue
		}
		partAmount := remaining
		if partAmount > available {
			partAmount = available
		}
		p.ReservedCents += partAmount
		p.UpdatedAt = now()
		if err = put(ctx, tx, "wallet_topups", p.ID, p); err != nil {
			return nil, err
		}
		v.Parts = append(v.Parts, DepositRefundPart{TopupID: p.ID, RefundNo: "DR" + strings.ToUpper(walletKey(id, p.ID)[:62]), AmountCents: partAmount, Status: "queued"})
		remaining -= partAmount
		if remaining == 0 {
			break
		}
	}
	if err = claimWalletRequest(ctx, tx, u.ID, in.RequestID, "deposit_refund", map[string]any{"amount_cents": amount, "scope": in.Scope, "club_id": in.ClubID}); err != nil {
		return nil, err
	}
	if remaining != 0 {
		return nil, fail(409, "押金充值来源无法核对，请联系平台核账")
	}
	if err = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "deposit_refund_freeze", ReferenceID: id, AmountCents: amount, DepositDelta: -amount, DepositRefundingDelta: amount, Note: "申请原路退还押金，退还中不计有效押金"}); err != nil {
		return nil, err
	}
	if err = put(ctx, tx, "wallet_deposit_refunds", id, v); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err = a.processDepositRefund(ctx, v); err != nil {
		return nil, err
	}
	return get[WalletDepositRefund](ctx, a.DB, "wallet_deposit_refunds", id, false)
}

// processDepositRefund 逐笔提交或查询原充值退款；任何超时或未知状态保留剩余冻结额。
func (a *App) processDepositRefund(ctx context.Context, snapshot WalletDepositRefund) error {
	var c wxPayConfig
	var err error
	if !a.Config.DevMode {
		c, err = a.loadWXPay()
		if err != nil {
			return err
		}
	}
	for index := range snapshot.Parts {
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			return err
		}
		if err = lockWalletUsers(ctx, tx, snapshot.UserID, refundAccount(snapshot)); err != nil {
			tx.Rollback(ctx)
			return err
		}
		current, err := get[WalletDepositRefund](ctx, tx, "wallet_deposit_refunds", snapshot.ID, true)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		part := current.Parts[index]
		if part.Status == "success" {
			tx.Rollback(ctx)
			continue
		}
		first := part.Status == "queued"
		if first {
			current.Parts[index].Status = "processing"
			current.UpdatedAt = now()
			err = put(ctx, tx, "wallet_deposit_refunds", current.ID, current)
		}
		p, readErr := get[WalletTopup](ctx, tx, "wallet_topups", part.TopupID, false)
		if err == nil {
			err = readErr
		}
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		if a.Config.DevMode {
			if err = a.applyDepositRefund(ctx, c, wxRefund{OrderNo: p.OrderNo, OutRefundNo: part.RefundNo, TransactionID: p.TransactionID, Status: "SUCCESS", Amount: struct {
				Total    int64  `json:"total"`
				Refund   int64  `json:"refund"`
				Currency string `json:"currency"`
			}{p.AmountCents, part.AmountCents, "CNY"}}, true); err != nil {
				return err
			}
			continue
		}
		var result wxRefund
		if !first {
			err = a.wxRequest(ctx, c, "GET", "/v3/refund/domestic/refunds/"+url.PathEscape(part.RefundNo), nil, &result)
			if err != nil {
				var apiErr *wxAPIError
				if !errors.As(err, &apiErr) || !contains([]string{"RESOURCE_NOT_EXISTS", "NOT_FOUND"}, apiErr.Code) {
					return err
				}
				first = true
			}
		}
		if first {
			payload := map[string]any{"transaction_id": p.TransactionID, "out_refund_no": part.RefundNo, "reason": "自愿平台押金原路退还", "notify_url": c.RefundNotifyURL, "amount": map[string]any{"refund": part.AmountCents, "total": p.AmountCents, "currency": "CNY"}}
			if err = a.wxRequest(ctx, c, "POST", "/v3/refund/domestic/refunds", payload, &result); err != nil {
				return err
			}
		}
		if err = a.applyDepositRefund(ctx, c, result, false); err != nil {
			return err
		}
	}
	return nil
}

// applyDepositRefund 校验每笔原支付与部分退款金额，成功后仅扣该笔冻结押金，支持重复和乱序通知。
func (a *App) applyDepositRefund(ctx context.Context, c wxPayConfig, result wxRefund, dev bool) error {
	if !dev && result.MerchantID != "" && result.MerchantID != c.MerchantID {
		return fail(400, "押金退款商户不匹配")
	}
	status := result.RefundStatus
	if status == "" {
		status = result.Status
	}
	if !contains([]string{"SUCCESS", "ABNORMAL", "CLOSED", "PROCESSING"}, status) {
		return fail(400, "押金退款尚未完成")
	}
	var id string
	if err := a.DB.QueryRow(ctx, "SELECT id FROM wallet_deposit_refunds WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(data->'parts') part WHERE part->>'refund_no'=$1)", result.OutRefundNo).Scan(&id); err != nil {
		return fail(404, "押金退款申请不存在")
	}
	snapshot, err := get[WalletDepositRefund](ctx, a.DB, "wallet_deposit_refunds", id, false)
	if err != nil {
		return err
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockWalletUsers(ctx, tx, snapshot.UserID, refundAccount(snapshot)); err != nil {
		return err
	}
	v, err := get[WalletDepositRefund](ctx, tx, "wallet_deposit_refunds", id, true)
	if err != nil {
		return err
	}
	index := -1
	for i, part := range v.Parts {
		if part.RefundNo == result.OutRefundNo {
			index = i
			break
		}
	}
	if index < 0 {
		return fail(404, "押金退款分笔不存在")
	}
	part := v.Parts[index]
	p, err := get[WalletTopup](ctx, tx, "wallet_topups", part.TopupID, true)
	if err != nil {
		return err
	}
	if p.Purpose == "balance" || p.UserID != v.UserID || topupAccount(p) != refundAccount(v) {
		return fail(409, "押金退款来源与申请用户不一致，请平台核账")
	}
	if result.OrderNo != p.OrderNo || result.TransactionID != p.TransactionID || p.TransactionID == "" || result.Amount.Total != p.AmountCents || result.Amount.Refund != part.AmountCents || (result.Amount.Currency != "" && result.Amount.Currency != "CNY") {
		return fail(400, "押金退款原支付或部分退款金额不匹配")
	}
	if part.Status == "success" {
		return nil
	}
	if status != "SUCCESS" {
		return nil
	}
	wallet, err := lockedWallet(ctx, tx, refundAccount(v))
	if err != nil {
		return err
	}
	if err = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "deposit_refund_success", ReferenceID: part.RefundNo, AmountCents: -part.AmountCents, DepositRefundingDelta: -part.AmountCents, Note: "微信已确认该笔押金原路退还"}); err != nil {
		return err
	}
	if p.ReservedCents < part.AmountCents || p.RefundedCents+part.AmountCents > p.AmountCents {
		return fail(409, "押金退款来源分账异常")
	}
	p.ReservedCents -= part.AmountCents
	p.RefundedCents += part.AmountCents
	p.UpdatedAt = now()
	if p.RefundedCents == p.AmountCents {
		p.Status = "refunded"
	}
	if err = put(ctx, tx, "wallet_topups", p.ID, p); err != nil {
		return err
	}
	v.Parts[index].Status = "success"
	v.Status = "success"
	for _, item := range v.Parts {
		if item.Status != "success" {
			v.Status = "processing"
		}
	}
	v.UpdatedAt = now()
	if err = put(ctx, tx, "wallet_deposit_refunds", id, v); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// syncWalletDepositRefund 仅本人可触发退款查询与稳定退款号补偿，不能指定任意付款来源。
func (a *App) syncWalletDepositRefund(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	v, err := get[WalletDepositRefund](r.Context(), a.DB, "wallet_deposit_refunds", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if v.UserID != u.ID {
		return nil, fail(403, "无权同步他人押金退款")
	}
	if v.Status != "success" {
		if err = a.processDepositRefund(r.Context(), v); err != nil {
			return nil, err
		}
	}
	return get[WalletDepositRefund](r.Context(), a.DB, "wallet_deposit_refunds", v.ID, false)
}

// refundAccount 兼容旧个人记录并按冻结时绑定的押金账户完成原路退款。
func refundAccount(v WalletDepositRefund) string {
	if v.AccountID != "" {
		return v.AccountID
	}
	return v.UserID
}
