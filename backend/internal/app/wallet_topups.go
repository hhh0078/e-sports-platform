// Package app 为充值余额和独立平台押金建立幂等支付意图，只有已验证付款才能增加资金。
package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// walletTopups 仅返回本人充值记录，支付秘密由统一响应层剥除。
func (a *App) walletTopups(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	return list[WalletTopup](r.Context(), a.DB, "wallet_topups", "WHERE data->>'user_id'=$1 ORDER BY (data->>'created_at')::timestamptz DESC,id DESC LIMIT $2 OFFSET $3", u.ID, limit, offset)
}

// createWalletTopup 同一账号和幂等号只能对应相同用途、相同金额，微信超时仍保留付款意图。
func (a *App) createWalletTopup(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if err = a.requireVerifiedPhone(r, u); err != nil {
		return nil, err
	}
	var in struct {
		Purpose     string `json:"purpose"`
		AmountCents int64  `json:"amount_cents"`
		RequestID   string `json:"request_id"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if err = walletRequestID(in.RequestID); err != nil {
		return nil, err
	}
	if !contains([]string{"balance", "deposit"}, in.Purpose) || in.AmountCents < 100 || in.AmountCents > 10000000 {
		return nil, fail(400, "充值用途无效或金额不在1至100000元范围内")
	}
	var paymentConfig wxPayConfig
	if !a.Config.DevMode {
		if paymentConfig, err = a.loadWXPay(); err != nil {
			return nil, err
		}
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockWalletUsers(ctx, tx, u.ID); err != nil {
		return nil, err
	}
	if err = claimWalletRequest(ctx, tx, u.ID, in.RequestID, "topup", map[string]any{"purpose": in.Purpose, "amount_cents": in.AmountCents}); err != nil {
		return nil, err
	}
	id := walletKey("topup", u.ID, in.RequestID)
	payment, err := get[WalletTopup](ctx, tx, "wallet_topups", id, true)
	if missing(err) {
		payment = WalletTopup{ID: id, UserID: u.ID, RequestID: in.RequestID, Purpose: in.Purpose, AmountCents: in.AmountCents, Status: "pending_payment", OrderNo: "WT" + strings.ToUpper(id[:30]), CreatedAt: now(), UpdatedAt: now()}
		if !a.Config.DevMode {
			identity, e := get[map[string]string](ctx, tx, "identities", u.ID, false)
			if e != nil || identity["openid"] == "" {
				return nil, fail(503, "请先通过微信登录后充值")
			}
			payment.PayOpenID = identity["openid"]
			payment.MerchantID = paymentConfig.MerchantID
			payment.AppID = paymentConfig.AppID
		}
		if err = put(ctx, tx, "wallet_topups", id, payment); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if payment.AmountCents != in.AmountCents || payment.Purpose != in.Purpose {
		return nil, fail(409, "同一request_id不能修改充值金额或用途")
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a.walletTopupPayment(r, u, payment)
}

// walletTopupPayment 基于已保存的原充值流水继续支付，客户端丢失幂等键也不会创建另一笔充值。
func (a *App) walletTopupPayment(r *http.Request, u User, payment WalletTopup) (any, error) {
	ctx := r.Context()
	var err error
	if a.Config.DevMode {
		if err = a.applyWalletTopup(ctx, payment, wxTransaction{TransactionID: "DEV" + payment.ID}, true); err != nil {
			return nil, err
		}
		payment, err = get[WalletTopup](ctx, a.DB, "wallet_topups", payment.ID, false)
		return map[string]any{"payment": payment, "mode": "dev"}, err
	}
	if payment.Status != "pending_payment" {
		return map[string]any{"payment": payment, "mode": "wechat"}, nil
	}
	c, err := a.loadWXPay()
	if err != nil {
		return nil, err
	}
	if payment.MerchantID != "" && (payment.MerchantID != c.MerchantID || payment.AppID != c.AppID) {
		return nil, fail(409, "支付商户配置已变更，请联系平台核查原充值意图，勿重新付款")
	}
	result, err := a.prepay(r, u, Order{ID: payment.ID, OrderNo: payment.OrderNo, Title: map[string]string{"balance": "开机电竞钱包充值", "deposit": "开机电竞自愿平台押金"}[payment.Purpose], AmountCents: payment.AmountCents, PayOpenID: payment.PayOpenID})
	if err != nil {
		return nil, err
	}
	response := result.(map[string]any)
	return map[string]any{"payment": payment, "mode": "wechat", "pay_params": response["pay_params"]}, nil
}

// applyWalletTopup 在付款信息完成上游核验后，用数据库锁和账本业务号保证只入账一次。
func (a *App) applyWalletTopup(ctx context.Context, snapshot WalletTopup, v wxTransaction, dev bool) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockWalletUsers(ctx, tx, snapshot.UserID); err != nil {
		return err
	}
	p, err := get[WalletTopup](ctx, tx, "wallet_topups", snapshot.ID, true)
	if err != nil {
		return err
	}
	if !dev && ((p.MerchantID != "" && (p.MerchantID != v.MerchantID || p.AppID != v.AppID)) || p.AmountCents != v.Amount.Total || p.PayOpenID == "" || p.PayOpenID != v.Payer.OpenID || p.OrderNo != v.OrderNo) {
		return fail(400, "充值金额或付款人不匹配")
	}
	if p.TransactionID != "" {
		if p.TransactionID != v.TransactionID {
			return fail(409, "充值支付流水冲突")
		}
		return nil
	}
	if p.Status != "pending_payment" {
		return fail(409, "充值状态异常，请平台核查")
	}
	wallet, err := lockedWallet(ctx, tx, p.UserID)
	if err != nil {
		return err
	}
	entry := WalletEntry{Kind: "topup", ReferenceID: p.ID, AmountCents: p.AmountCents, Note: "充值余额，不支持提现"}
	if p.Purpose == "deposit" {
		entry.DepositDelta = p.AmountCents
		entry.Note = "自愿缴纳平台押金，独立保管，可申请原路退还"
	} else {
		entry.RechargeDelta = p.AmountCents
	}
	if err = appendWalletEntry(ctx, tx, &wallet, entry); err != nil {
		return err
	}
	p.Status = "paid"
	p.TransactionID = v.TransactionID
	p.UpdatedAt = now()
	if err = put(ctx, tx, "wallet_topups", p.ID, p); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// syncWalletTopup 查询本人充值的官方付款结果，客户端支付完成事件不直接入账。
func (a *App) syncWalletTopup(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	p, err := get[WalletTopup](r.Context(), a.DB, "wallet_topups", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if p.UserID != u.ID {
		return nil, fail(403, "无权同步他人充值")
	}
	if !a.Config.DevMode && p.Status == "pending_payment" {
		c, e := a.loadWXPay()
		if e != nil {
			return nil, e
		}
		var result wxTransaction
		if err = a.wxRequest(r.Context(), c, "GET", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(p.OrderNo)+"?mchid="+url.QueryEscape(c.MerchantID), nil, &result); err != nil {
			return nil, err
		}
		if result.TradeState == "SUCCESS" {
			if err = a.applyWXPayment(r.Context(), c, result); err != nil {
				return nil, err
			}
		}
	}
	return get[WalletTopup](r.Context(), a.DB, "wallet_topups", p.ID, false)
}

// resumeWalletTopup 仅本人可按原充值编号恢复未付款意图，已付款只返回现有结果。
func (a *App) resumeWalletTopup(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if err = a.requireVerifiedPhone(r, u); err != nil {
		return nil, err
	}
	p, err := get[WalletTopup](r.Context(), a.DB, "wallet_topups", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if p.UserID != u.ID {
		return nil, fail(403, "无权支付他人充值记录")
	}
	return a.walletTopupPayment(r, u, p)
}
