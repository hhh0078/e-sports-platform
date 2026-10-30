// Package app 把业务订单与官方代币扣减绑定，先保留资金意图再出网，未知扣币重用原单恢复。
package app

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// virtualConsumptionID 为初付款及每笔补款生成独立稳定扣币业务号。
func virtualConsumptionID(orderID, adjustmentID string) string {
	return walletKey("virtual-consumption", orderID, adjustmentID)
}

// useVirtualPayment 只为小程序新服务付款选择虚拟支付，已经发起的原普通微信单始终保持原渠道。
func (a *App) useVirtualPayment(o Order) bool {
	return !a.Config.DevMode && (o.PaymentChannel == "virtual" || o.PayOpenID == "" && o.PaymentChannel == "" && a.commerceSettings().Mode == "mini_program")
}

// prepareVirtualConsumption 在订单及用户锁中冻结收费快照，每个用户只允许一笔未知扣币，防止余额重复占用。
func (a *App) prepareVirtualConsumption(ctx context.Context, q querier, u User, o Order, adjustmentID, platform, userIP string, amount, expectedTotal, expectedTopup int64) (virtualConsumption, error) {
	id := virtualConsumptionID(o.ID, adjustmentID)
	p, e := get[virtualConsumption](ctx, q, "virtual_consumptions", id, true)
	if e == nil {
		if p.UserID != u.ID || p.Quote.ServiceCents != amount || p.Status == "cancelled" {
			return p, fail(409, "原金币支付意图与当前订单不一致")
		}
		return p, nil
	}
	if !missing(e) {
		return p, e
	}
	if a.Config.WechatMessageToken == "" {
		return p, fail(503, "虚拟支付消息验签尚未配置，请先接通官方充值和Apple退款通知")
	}
	var active bool
	if e = q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM virtual_consumptions WHERE data->>'user_id'=$1 AND data->>'status' IN ('awaiting_funds','debit_pending'))", u.ID).Scan(&active); e != nil {
		return p, e
	}
	if active {
		return p, fail(409, "另有金币付款尚未确认，请先同步或取消原订单")
	}
	c := a.commerceSettings()
	if _, _, e = a.virtualConfig(a.Config.WechatAppID, c.OfferID, c.VirtualEnv); e != nil {
		return p, e
	}
	s, e := a.miniProgramSessionWith(ctx, q, u.ID)
	if e != nil {
		return p, e
	}
	if _, _, e = a.virtualConfig(s.AppID, c.OfferID, c.VirtualEnv); e != nil {
		return p, e
	}
	wallet, e := lockedWallet(ctx, q, u.ID)
	if e != nil {
		return p, e
	}
	requestPlatform := platform
	platform, e = virtualFeePlatform(ctx, q, u.ID, c, platform)
	if e != nil {
		return p, e
	}
	quote, e := makeVirtualQuote(c, amount, wallet.VirtualCoinCents, platform)
	if e != nil {
		return p, e
	}
	if expectedTotal <= 0 || expectedTopup < 0 || expectedTotal != quote.TotalCents || expectedTopup != quote.TopupCents {
		return p, fail(409, "金币余额或手续费已变化，请重新确认支付报价")
	}
	p = virtualConsumption{ID: id, OrderID: o.ID, AdjustmentID: adjustmentID, UserID: u.ID, AppID: s.AppID, OpenID: s.OpenID, OfferID: c.OfferID, Env: c.VirtualEnv, OrderNo: "VP" + strings.ToUpper(id[:30]), UserIP: userIP, Quote: quote, Status: "awaiting_funds", CreatedAt: now(), RequestPlatform: requestPlatform}
	if quote.TopupCents > 0 {
		p.TopupID = walletKey("virtual-shortfall", id)
		if _, e = a.newVirtualTopup(ctx, q, u, p.TopupID, "virtual-"+id[:32], quote.TopupCents); e != nil {
			return p, e
		}
	}
	return p, put(ctx, q, "virtual_consumptions", id, p)
}

// virtualOrderPayment 先核验可能已到账的原充值，再进行官方扣币或返回同一现金充值会话。
func (a *App) virtualOrderPayment(r *http.Request, u User, p virtualConsumption) (any, error) {
	shouldSync := true
	if p.Status == "awaiting_funds" && p.TopupID != "" {
		topup, e := get[WalletTopup](r.Context(), a.DB, "wallet_topups", p.TopupID, false)
		if e != nil {
			return nil, e
		}
		shouldSync = topup.Status == "paid"
	}
	if shouldSync {
		if e := a.syncVirtualConsumption(r.Context(), p); e != nil {
			return nil, e
		}
	}
	current, e := get[virtualConsumption](r.Context(), a.DB, "virtual_consumptions", p.ID, false)
	if e != nil {
		return nil, e
	}
	o, e := get[Order](r.Context(), a.DB, "orders", p.OrderID, false)
	if e != nil {
		return nil, e
	}
	response := map[string]any{"order": o, "mode": "virtual", "virtual_quote": current.Quote}
	if p.AdjustmentID != "" {
		v, e := get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", p.AdjustmentID, false)
		if e != nil {
			return nil, e
		}
		response["adjustment"] = v
	}
	if current.Status == "awaiting_funds" && current.TopupID != "" {
		topup, e := get[WalletTopup](r.Context(), a.DB, "wallet_topups", current.TopupID, false)
		if e != nil {
			return nil, e
		}
		if topup.Status == "pending_payment" {
			v, e := a.virtualTopupPayment(r, u, topup)
			if e != nil {
				return nil, e
			}
			response["virtual_params"] = v.(map[string]any)["virtual_params"]
			response["payment"] = topup
		}
	}
	return response, nil
}

// syncVirtualConsumption 只在官方余额和本地来源均足够时发起扣币，超时后不重新充值或生成第二个扣币号。
func (a *App) syncVirtualConsumption(ctx context.Context, snapshot virtualConsumption) error {
	if snapshot.Status == "paid" || snapshot.Status == "cancelled" {
		return nil
	}
	if snapshot.TopupID != "" && snapshot.Status == "awaiting_funds" {
		p, e := get[WalletTopup](ctx, a.DB, "wallet_topups", snapshot.TopupID, false)
		if e != nil {
			return e
		}
		if p.Status != "paid" {
			if e = a.syncVirtualTopup(ctx, p); e != nil {
				return e
			}
			p, e = get[WalletTopup](ctx, a.DB, "wallet_topups", p.ID, false)
			if e != nil {
				return e
			}
			if p.Status != "paid" {
				return nil
			}
		}
		evidence, e := get[virtualTransaction](ctx, a.DB, "virtual_transactions", p.ID, false)
		if e != nil {
			return e
		}
		actual := "android"
		if evidence.Order.OrderType == 7 {
			actual = "ios"
		}
		if actual != snapshot.RequestPlatform {
			return fail(409, "官方充值渠道与原报价不一致，请取消业务意图后按已核验金币来源重新报价；金币保留本人账户")
		}
	}
	s, e := a.miniProgramSession(ctx, snapshot.UserID)
	if e != nil {
		return e
	}
	if s.AppID != snapshot.AppID || s.OpenID != snapshot.OpenID {
		return fail(409, "金币扣币原身份不一致，请使用原小程序账号同步")
	}
	if snapshot.Status == "awaiting_funds" {
		if e = a.refreshVirtualFunding(ctx, snapshot.UserID, "virtual_recharge"); e != nil {
			return e
		}
		var balance struct {
			Balance *int64 `json:"balance"`
		}
		code, e := a.virtualRequest(ctx, s.AppID, snapshot.OfferID, snapshot.Env, "/xpay/query_user_balance", map[string]any{"openid": s.OpenID, "env": snapshot.Env, "user_ip": snapshot.UserIP}, s.SessionKey, &balance)
		if e != nil {
			return e
		}
		if code != 0 {
			return virtualResultError(code)
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
		p, e := get[virtualConsumption](ctx, tx, "virtual_consumptions", snapshot.ID, true)
		if e != nil {
			return e
		}
		if p.Status == "paid" {
			return nil
		}
		if p.Status != "awaiting_funds" && p.Status != "debit_pending" {
			return fail(409, "金币支付状态异常")
		}
		if p.Status == "awaiting_funds" {
			if e = requireAdultIdentity(ctx, tx, p.UserID); e != nil {
				return e
			}
			actor, e := get[User](ctx, tx, "users", p.UserID, false)
			if e != nil {
				return e
			}
			if actor.Status != "active" {
				return fail(409, "付款账号不可用，已购金币仍归原账号保管")
			}
			if p.AdjustmentID == "" {
				if o.Status != "pending_payment" || o.PaymentStatus != "unpaid" {
					return fail(409, "订单已结束，已购金币保留在原账户")
				}
			} else {
				v, e := get[OrderAdjustment](ctx, tx, "order_adjustments", p.AdjustmentID, true)
				if e != nil {
					return e
				}
				deadline, pe := time.Parse(time.RFC3339Nano, v.ExpiresAt)
				if pe != nil || !time.Now().Before(deadline) || o.ActiveAdjustmentID != v.ID || v.Status != "paying" {
					return fail(409, "补款已到期或已结束，已购金币保留在原账户")
				}
			}
			wallet, e := lockedWallet(ctx, tx, p.UserID)
			if e != nil {
				return e
			}
			if balance.Balance == nil || *balance.Balance < 0 || *balance.Balance > 2000000000 || *balance.Balance*100 < wallet.VirtualCoinCents || wallet.VirtualCoinCents < p.Quote.TotalCents {
				return fail(409, "官方金币余额与已核验来源不一致，请同步原充值或联系平台核账")
			}
			p.Status = "debit_pending"
			if e = put(ctx, tx, "virtual_consumptions", p.ID, p); e != nil {
				return e
			}
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
		snapshot = p
	}
	code, e := a.virtualRequest(ctx, s.AppID, snapshot.OfferID, snapshot.Env, "/xpay/currency_pay", map[string]any{"openid": s.OpenID, "env": snapshot.Env, "user_ip": snapshot.UserIP, "amount": snapshot.Quote.TotalCents / 100, "order_id": snapshot.OrderNo, "payitem": "[]", "remark": "开机电竞服务订单"}, s.SessionKey, nil)
	if e != nil {
		return e
	}
	if e = virtualResultError(code); e != nil {
		return e
	}
	return a.applyVirtualConsumption(ctx, snapshot)
}

// accountVirtualCoins 从已核验的金币批次扣除服务额，保留每一份原现金充值来源用于退款和提现。
func accountVirtualCoins(ctx context.Context, q querier, userID, consumptionID, orderID string, amount int64) error {
	wallet, e := lockedWallet(ctx, q, userID)
	if e != nil {
		return e
	}
	if e = appendWalletEntry(ctx, q, &wallet, WalletEntry{Kind: "virtual_payment", ReferenceID: consumptionID, AmountCents: -amount, VirtualDelta: -amount, Note: "官方金币支付服务，原充值来源逐笔保留"}); e != nil {
		return e
	}
	return moveFunding(ctx, q, "virtual-pay:"+consumptionID, userID, "virtual_recharge", amount, []fundDestination{{AccountID: "escrow:" + orderID, Bucket: "escrow", OrderID: orderID, AmountCents: amount, PreserveReturn: true}})
}

// applyVirtualConsumption 在官方扣币成功后提交托管与服务收入，手续费独立保管，不计入GMV和服务分账。
func (a *App) applyVirtualConsumption(ctx context.Context, snapshot virtualConsumption) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	o, e := lockWalletOrder(ctx, tx, snapshot.OrderID, "")
	if e != nil {
		return e
	}
	o.HasVirtualPayment = true
	p, e := get[virtualConsumption](ctx, tx, "virtual_consumptions", snapshot.ID, true)
	if e != nil {
		return e
	}
	if p.Status == "paid" {
		return nil
	}
	if p.Status != "debit_pending" {
		return fail(409, "官方扣币成功但原意图异常，请平台核账")
	}
	if p.Quote.FeeCents > 0 {
		wallet, e := lockedWallet(ctx, tx, p.UserID)
		if e != nil {
			return e
		}
		if e = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "virtual_fee", ReferenceID: p.ID, AmountCents: -p.Quote.FeeCents, VirtualDelta: -p.Quote.FeeCents, Note: "买家承担的订单金币服务费"}); e != nil {
			return e
		}
		if e = moveFunding(ctx, tx, "virtual-fee:"+p.ID, p.UserID, "virtual_recharge", p.Quote.FeeCents, []fundDestination{{AccountID: "virtual-fee:" + p.ID, Bucket: "virtual_fee", OrderID: o.ID, AmountCents: p.Quote.FeeCents, PreserveReturn: true}}); e != nil {
			return e
		}
	}
	if p.AdjustmentID == "" {
		if o.Status != "pending_payment" || o.PaymentStatus != "unpaid" || o.BuyerID != p.UserID || o.AmountCents != p.Quote.ServiceCents {
			return fail(409, "扣币原订单状态冲突，请平台核账")
		}
		o.PaymentTransactionID = p.OrderNo
		o.PaymentMerchantID = "virtual:" + p.OfferID
		o.PaymentAppID = p.AppID
		o.PayOpenID = p.OpenID
		if e = accountWalletPayment(ctx, tx, &o, "virtual", p.ID); e != nil {
			return e
		}
		o.Status = "open"
		o.PaymentStatus = "paid"
		o.UpdatedAt = now()
		o.AdjustmentVersion = 1
		o.OriginalAmountCents = o.AmountCents
		o.PaidTotalCents = o.AmountCents
		base := OrderPayment{ID: walletKey("base-order-payment", o.ID), OrderID: o.ID, BuyerID: o.BuyerID, Kind: "base", Channel: "virtual", AmountCents: o.AmountCents, Status: "paid", OrderNo: o.OrderNo, TransactionID: p.OrderNo, MerchantID: o.PaymentMerchantID, AppID: p.AppID, PayOpenID: p.OpenID, CreatedAt: o.PaidAt, PaidAt: o.PaidAt, UpdatedAt: now()}
		if e = put(ctx, tx, "order_payments", base.ID, base); e != nil {
			return e
		}
		if e = put(ctx, tx, "orders", o.ID, o); e != nil {
			return e
		}
		if e = systemMessage(ctx, tx, o, "微信官方金币扣款已核验，款项已托管，等待打手接单。"); e != nil {
			return e
		}
	} else {
		v, e := get[OrderAdjustment](ctx, tx, "order_adjustments", p.AdjustmentID, true)
		if e != nil {
			return e
		}
		payment, e := get[OrderPayment](ctx, tx, "order_payments", adjustmentPaymentID(v.ID), true)
		if e != nil {
			return e
		}
		payment.TransactionID = p.OrderNo
		payment.MerchantID = "virtual:" + p.OfferID
		payment.AppID = p.AppID
		payment.PayOpenID = p.OpenID
		if e = accountAdjustmentPayment(ctx, tx, &o, &v, &payment, "virtual"); e != nil {
			return e
		}
	}
	p.Status = "paid"
	p.PaidAt = now()
	if e = put(ctx, tx, "virtual_consumptions", p.ID, p); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// cancelVirtualPayment 只在尚未请求官方扣币时取消业务意图，已购金币仍归用户，未知扣币不解冻。
func cancelVirtualPayment(ctx context.Context, q querier, o Order) error {
	p, e := get[virtualConsumption](ctx, q, "virtual_consumptions", virtualConsumptionID(o.ID, ""), true)
	if missing(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if p.Status == "debit_pending" || p.Status == "paid" {
		return fail(409, "官方金币扣币结果待确认，请先同步原订单")
	}
	p.Status = "cancelled"
	return put(ctx, q, "virtual_consumptions", p.ID, p)
}

// virtualClientIP 仅使用服务器实际连接地址，不接受客户端自报公网地址改变签名请求。
func virtualClientIP(r *http.Request) string {
	ip, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil || net.ParseIP(ip) == nil {
		return "127.0.0.1"
	}
	return ip
}

// expireVirtualAdjustment 与扣币共用订单和用户锁，只有仍未发起官方扣币的原意图才能过期。
func (a *App) expireVirtualAdjustment(ctx context.Context, orderID, adjustmentID string) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	o, e := lockWalletOrder(ctx, tx, orderID, "")
	if e != nil {
		return e
	}
	p, e := get[virtualConsumption](ctx, tx, "virtual_consumptions", virtualConsumptionID(orderID, adjustmentID), true)
	if e != nil {
		return e
	}
	if p.Status != "awaiting_funds" {
		return fail(409, "官方扣币已开始或已完成，请同步原单，不能取消在途资金")
	}
	v, e := get[OrderAdjustment](ctx, tx, "order_adjustments", adjustmentID, true)
	if e != nil {
		return e
	}
	deadline, pe := time.Parse(time.RFC3339Nano, v.ExpiresAt)
	if pe != nil || time.Now().Before(deadline) || o.ActiveAdjustmentID != v.ID || v.Status != "paying" {
		return fail(409, "原补款状态已变化，请重新同步")
	}
	p.Status = "cancelled"
	v.Status = "expired"
	v.UpdatedAt = now()
	o.ActiveAdjustmentID = ""
	if e = put(ctx, tx, "virtual_consumptions", p.ID, p); e != nil {
		return e
	}
	if e = put(ctx, tx, "order_adjustments", v.ID, v); e != nil {
		return e
	}
	if e = put(ctx, tx, "orders", o.ID, o); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// virtualFeePlatform 以已核验现金来源提高费用下限，伪报低费终端不能降低既有金币的真实渠道费用。
func virtualFeePlatform(ctx context.Context, q querier, userID string, c commerceSettings, requested string) (string, error) {
	if !contains([]string{"android", "ios"}, requested) {
		return "", fail(400, "请选择有效支付终端")
	}
	lots, e := list[FundingLot](ctx, q, "fund_lots", "WHERE data->>'account_id'=$1 AND data->>'bucket'='virtual_recharge' AND (data->>'remaining_cents')::bigint>0", userID)
	if e != nil {
		return "", e
	}
	selected := requested
	for _, l := range lots {
		receipt, e := get[FundingReceipt](ctx, q, "fund_receipts", l.ReceiptID, false)
		if e != nil {
			return "", e
		}
		proof, e := get[virtualTransaction](ctx, q, "virtual_transactions", receipt.PaymentID, false)
		if e != nil || proof.ReconciliationRequired {
			return "", fail(409, "金币现金来源待核账，暂不能消费")
		}
		if proof.Order.OrderType == 7 && c.IOSFeeBPS > c.AndroidFeeBPS {
			selected = "ios"
		}
		if proof.Order.OrderType == 0 && c.AndroidFeeBPS > c.IOSFeeBPS {
			selected = "android"
		}
	}
	return selected, nil
}
