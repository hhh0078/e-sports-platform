// Package app 验证虚拟支付消息签名与AES加密，现金退款撤回未消费金币并冻结已流转来源供平台核账。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"time"
)

// virtualCallbackEvent 仅解码官方已核对的充值、现金退款与Apple退款问询字段。
type virtualCallbackEvent struct {
	Event       string `json:"Event" xml:"Event"`
	OpenID      string `json:"OpenId" xml:"OpenId"`
	OutTradeNo  string `json:"OutTradeNo" xml:"OutTradeNo"`
	MchOrderID  string `json:"MchOrderId" xml:"MchOrderId"`
	WXOrderID   string `json:"WxOrderId" xml:"WxOrderId"`
	WXRefundID  string `json:"WxRefundId" xml:"WxRefundId"`
	RefundFee   int64  `json:"RefundFee" xml:"RefundFee"`
	RetCode     *int   `json:"RetCode" xml:"RetCode"`
	PayOrderID  string `json:"pay_order_id" xml:"pay_order_id"`
	RefundQuery struct {
		PayOrderID string `json:"pay_order_id" xml:"pay_order_id"`
	} `json:"WxaVirtualPayIosRefundQueryNotifyEvent" xml:"WxaVirtualPayIosRefundQueryNotifyEvent"`
}

// virtualPaymentCallback 只接受固定时间窗内的官方签名消息，禁止重定向或客户端支付完成事件触发资金记账。
func (a *App) virtualPaymentCallback(w http.ResponseWriter, r *http.Request) {
	if a.Config.DevMode || a.Config.WechatMessageToken == "" || a.Config.WechatAppID == "" {
		http.Error(w, "callback not configured", 503)
		return
	}
	q := r.URL.Query()
	stamp, e := strconv.ParseInt(q.Get("timestamp"), 10, 64)
	if e != nil || time.Since(time.Unix(stamp, 0)) > 10*time.Minute || time.Until(time.Unix(stamp, 0)) > time.Minute || len(q.Get("nonce")) > 256 {
		http.Error(w, "invalid timestamp", 403)
		return
	}
	var raw []byte
	if r.Method == http.MethodGet {
		raw = []byte(q.Get("echostr"))
	} else {
		raw, e = io.ReadAll(io.LimitReader(r.Body, 65537))
		if e != nil || len(raw) > 65536 {
			http.Error(w, "invalid body", 400)
			return
		}
	}
	encrypted := q.Get("encrypt_type") == "aes"
	if a.Config.WechatMessageAESKey != "" && !encrypted {
		http.Error(w, "encrypted callback required", 403)
		return
	}
	if encrypted {
		encoded := string(raw)
		if r.Method != http.MethodGet {
			var envelope struct {
				Encrypt string `json:"Encrypt" xml:"Encrypt"`
			}
			if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
				e = json.Unmarshal(raw, &envelope)
			} else {
				e = xml.Unmarshal(raw, &envelope)
			}
			if e != nil || envelope.Encrypt == "" {
				http.Error(w, "invalid envelope", 400)
				return
			}
			encoded = envelope.Encrypt
		}
		if !validSubscriptionSignature(q.Get("msg_signature"), a.Config.WechatMessageToken, q.Get("timestamp"), q.Get("nonce"), encoded) {
			http.Error(w, "invalid signature", 403)
			return
		}
		raw, e = decryptSubscriptionMessage(encoded, a.Config.WechatMessageAESKey, a.Config.WechatAppID)
		if e != nil {
			http.Error(w, "invalid message", 403)
			return
		}
	} else if !validSubscriptionSignature(q.Get("signature"), a.Config.WechatMessageToken, q.Get("timestamp"), q.Get("nonce")) {
		http.Error(w, "invalid signature", 403)
		return
	}
	if r.Method == http.MethodGet {
		w.Write(raw)
		return
	}
	if !a.handleVirtualCallback(w, r, raw) {
		http.Error(w, "unsupported event", 400)
	}
}

// handleVirtualCallback 在既有消息验签之后分流虚拟事件；问询在三秒内应答，最终现金退款必须另行核验。
func (a *App) handleVirtualCallback(w http.ResponseWriter, r *http.Request, raw []byte) bool {
	jsonFormat := bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{"))
	var v virtualCallbackEvent
	var e error
	if jsonFormat {
		e = json.Unmarshal(raw, &v)
	} else {
		e = xml.Unmarshal(raw, &v)
	}
	if e != nil || !contains([]string{"xpay_coin_pay_notify", "xpay_refund_notify", "xpay_subscribe_ios_refund_query_notify"}, v.Event) {
		return false
	}
	if v.Event == "xpay_subscribe_ios_refund_query_notify" {
		payOrderID := v.RefundQuery.PayOrderID
		if payOrderID == "" {
			payOrderID = v.PayOrderID
		}
		advice := a.virtualIOSRefundAdvice(r.Context(), payOrderID)
		if jsonFormat {
			respond(w, 200, advice)
		} else {
			w.Header().Set("Content-Type", "application/xml")
			xml.NewEncoder(w).Encode(advice)
		}
		return true
	}
	var id string
	e = a.DB.QueryRow(r.Context(), "SELECT id FROM wallet_topups WHERE data->>'channel'='virtual' AND (($1<>'' AND data->>'order_no'=$1) OR ($2<>'' AND data->>'order_no'=$2) OR ($3<>'' AND data->>'transaction_id'=$3)) LIMIT 1", v.OutTradeNo, v.MchOrderID, v.WXOrderID).Scan(&id)
	if e == nil {
		var p WalletTopup
		p, e = get[WalletTopup](r.Context(), a.DB, "wallet_topups", id, false)
		if e == nil && v.OpenID != "" && v.OpenID != p.PayOpenID {
			e = fail(409, "虚拟支付通知付款人不匹配")
		}
		if e == nil {
			if v.Event == "xpay_refund_notify" {
				if v.RetCode == nil || *v.RetCode != 0 || v.WXRefundID == "" || v.RefundFee <= 0 {
					e = fail(409, "现金退款通知尚未确认成功")
				} else {
					e = a.recordVirtualCashRefund(r.Context(), p, v.RefundFee, v.WXRefundID, false)
				}
			} else {
				e = a.syncVirtualTopup(r.Context(), p)
			}
		}
	}
	if e != nil {
		http.Error(w, "funds pending reconciliation", 503)
		return true
	}
	if jsonFormat {
		respond(w, 200, map[string]any{"ErrCode": 0, "ErrMsg": "success"})
	} else {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "<xml><ErrCode>0</ErrCode><ErrMsg>success</ErrMsg></xml>")
	}
	return true
}

// recordVirtualCashRefund 幂等撤回原现金充值尚留在本人账户的金币，已消费或已提现部分冻结来源并追加审计，不凭空冲销收入。
func (a *App) recordVirtualCashRefund(ctx context.Context, p WalletTopup, amount int64, eventID string, cumulative bool) error {
	if amount <= 0 || amount > p.AmountCents || eventID == "" {
		return fail(409, "现金退款金额或凭据无效")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = lockWalletUsers(ctx, tx, p.UserID); e != nil {
		return e
	}
	current, e := get[WalletTopup](ctx, tx, "wallet_topups", p.ID, true)
	if e != nil {
		return e
	}
	id := walletKey("virtual-cash-refund", p.ID, eventID)
	if _, e = get[map[string]any](ctx, tx, "virtual_transactions", id, false); e == nil {
		return nil
	} else if !missing(e) {
		return e
	}
	proof, e := get[virtualTransaction](ctx, tx, "virtual_transactions", p.ID, true)
	if missing(e) {
		proof = virtualTransaction{ID: p.ID, TopupID: p.ID}
	} else if e != nil {
		return e
	}
	// 查单返回累计额，通知返回单笔额；分别记录后取最大值，避免同一退款先查到再通知时重复扣币。
	notified := proof.NotifiedRefundCents
	target := amount
	if !cumulative {
		notified += amount
		target = notified
	}
	if target < current.RefundedCents {
		target = current.RefundedCents
	}
	if target > current.AmountCents {
		return fail(409, "累计现金退款超过原充值，来源保持冻结待核查")
	}
	delta := target - current.RefundedCents
	if delta < 0 {
		delta = 0
	}
	recovered := int64(0)
	if current.Status == "paid" && delta > 0 {
		lots, e := fundingLots(ctx, tx, p.UserID, "virtual_recharge")
		if e != nil {
			return e
		}
		movement := FundingMovement{ID: walletKey("fund-move", id), Inputs: []FundingPart{}, Outputs: []FundingPart{}, CreatedAt: now()}
		for _, l := range lots {
			if l.ReceiptID != walletKey("receipt", "topup", p.ID) {
				continue
			}
			take := l.RemainingCents
			if take > delta-recovered {
				take = delta - recovered
			}
			if take <= 0 {
				break
			}
			l.RemainingCents -= take
			recovered += take
			movement.Inputs = append(movement.Inputs, FundingPart{l.ID, take})
			if e = put(ctx, tx, "fund_lots", l.ID, l); e != nil {
				return e
			}
		}
		if recovered > 0 {
			wallet, e := lockedWallet(ctx, tx, p.UserID)
			if e != nil {
				return e
			}
			if e = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "virtual_cash_refund", ReferenceID: id, AmountCents: -recovered, VirtualDelta: -recovered, Note: "官方现金退款已确认，撤回尚未消费的对应金币"}); e != nil {
				return e
			}
			if e = put(ctx, tx, "fund_movements", movement.ID, movement); e != nil {
				return e
			}
		}
	} else if current.Status == "pending_payment" {
		current.Status = "cash_refunded"
	}
	current.RefundedCents = target
	current.UpdatedAt = now()
	proof.CashRefundCents = target
	proof.NotifiedRefundCents = notified
	proof.RecoveredCoinCents += recovered
	proof.ReconciliationRequired = true
	proof.CheckedAt = now()
	if e = put(ctx, tx, "wallet_topups", p.ID, current); e != nil {
		return e
	}
	if e = put(ctx, tx, "virtual_transactions", p.ID, proof); e != nil {
		return e
	}
	if e = put(ctx, tx, "virtual_transactions", id, map[string]any{"id": id, "kind": "virtual_cash_refund", "amount_cents": delta, "recovered_coin_cents": recovered, "status": "confirmed", "created_at": now()}); e != nil {
		return e
	}
	if e = audit(ctx, tx, "system", "virtual.cash_refund", p.ID, "官方现金退款已确认：原充值相关资金来源冻结；尚未消费金币已撤回，已流转款项须平台逐笔核账，不能增发可提收入"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
