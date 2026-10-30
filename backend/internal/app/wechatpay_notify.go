// Package app 验签解密微信支付通知，并在数据库事务中幂等更新付款与退款记录。
package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type wxTransaction struct {
	SuccessTime   string `json:"success_time"`
	AppID         string `json:"appid"`
	MerchantID    string `json:"mchid"`
	OrderNo       string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	Payer         struct {
		OpenID string `json:"openid"`
	} `json:"payer"`
	Amount struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}
type wxRefund struct {
	SuccessTime   string `json:"success_time"`
	MerchantID    string `json:"mchid"`
	OrderNo       string `json:"out_trade_no"`
	OutRefundNo   string `json:"out_refund_no"`
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	RefundStatus  string `json:"refund_status"`
	Amount        struct {
		Total    int64  `json:"total"`
		Refund   int64  `json:"refund"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

// decryptWXResource 校验资源类型并使用 APIv3 密钥解密认证密文。
func decryptWXResource(c wxPayConfig, raw []byte, expected string, out any) error {
	var envelope struct {
		EventType string `json:"event_type"`
		Resource  struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.EventType != expected || envelope.Resource.Algorithm != "AEAD_AES_256_GCM" {
		return fail(400, "微信通知类型无效")
	}
	block, err := aes.NewCipher([]byte(c.APIKey))
	if err != nil {
		return fail(503, "微信解密密钥无效")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	resource := envelope.Resource
	if len(resource.Nonce) != gcm.NonceSize() {
		return fail(400, "微信通知随机串无效")
	}
	data, err := base64.StdEncoding.DecodeString(resource.Ciphertext)
	if err != nil {
		return fail(400, "微信通知密文无效")
	}
	plain, err := gcm.Open(nil, []byte(resource.Nonce), data, []byte(resource.AssociatedData))
	if err != nil {
		return fail(401, "微信通知解密认证失败")
	}
	if json.Unmarshal(plain, out) != nil {
		return fail(400, "微信通知业务内容无效")
	}
	return nil
}

// readWXNotify 限制通知大小并先验签，未经认证的数据不能进入业务状态机。
func (a *App) readWXNotify(w http.ResponseWriter, r *http.Request) (wxPayConfig, []byte, error) {
	c, err := a.loadWXPay()
	if err != nil {
		return c, nil, err
	}
	if a.Config.DevMode {
		return c, nil, fail(409, "开发模式不接收真实支付回调")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return c, nil, fail(400, "支付回调内容过大")
	}
	if err = wxVerify(c, r.Header, body); err != nil {
		return c, nil, err
	}
	return c, body, nil
}

// paymentCallback 处理付款成功通知，数据库提交后才确认已接收。
func (a *App) paymentCallback(w http.ResponseWriter, r *http.Request) {
	c, body, err := a.readWXNotify(w, r)
	var transaction wxTransaction
	if err == nil {
		err = decryptWXResource(c, body, "TRANSACTION.SUCCESS", &transaction)
	}
	if err == nil {
		err = a.applyWXPayment(r.Context(), c, transaction)
	}
	wxNotifyResult(w, err)
}

// refundCallback 处理退款结果通知，成功与异常均保留可追踪的订单系统记录。
func (a *App) refundCallback(w http.ResponseWriter, r *http.Request) {
	c, body, err := a.readWXNotify(w, r)
	var refund wxRefund
	if err == nil {
		var header struct {
			EventType string `json:"event_type"`
		}
		if json.Unmarshal(body, &header) != nil || !contains([]string{"REFUND.SUCCESS", "REFUND.ABNORMAL", "REFUND.CLOSED"}, header.EventType) {
			err = fail(400, "退款通知类型无效")
		} else {
			err = decryptWXResource(c, body, header.EventType, &refund)
			if err == nil && header.EventType != "REFUND."+refund.RefundStatus {
				err = fail(400, "退款通知状态与事件不匹配")
			}
		}
	}
	if err == nil {
		if refund.MerchantID != c.MerchantID {
			err = fail(400, "退款通知商户不匹配")
		} else {
			err = a.applyWXRefund(r.Context(), c, refund)
		}
	}
	wxNotifyResult(w, err)
}

// wxNotifyResult 使用微信通知协议返回结果，失败时允许官方重试。
func wxNotifyResult(w http.ResponseWriter, err error) {
	if err != nil {
		status := 500
		message := "交易处理失败，请重试"
		if e, ok := err.(*apiError); ok {
			status = e.Status
			message = e.Message
		}
		respond(w, status, map[string]string{"code": "FAIL", "message": message})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// applyWXPayment 核对商户、买家、金额与原订单，并对重复支付通知幂等处理。
func (a *App) applyWXPayment(ctx context.Context, c wxPayConfig, v wxTransaction) error {
	original, e := a.loadWXPayForApp(v.AppID)
	if e != nil {
		return e
	}
	c = original
	if v.MerchantID != c.MerchantID || v.AppID != c.AppID || v.TradeState != "SUCCESS" || v.TransactionID == "" || v.Amount.Currency != "CNY" {
		return fail(400, "支付商户或交易信息不匹配")
	}
	if strings.HasPrefix(v.OrderNo, "OA") {
		return a.applyAdjustmentWXPayment(ctx, c, v)
	}
	if strings.HasPrefix(v.OrderNo, "WT") {
		var id string
		if err := a.DB.QueryRow(ctx, "SELECT id FROM wallet_topups WHERE data->>'order_no'=$1", v.OrderNo).Scan(&id); err != nil {
			return fail(404, "充值订单不存在")
		}
		p, err := get[WalletTopup](ctx, a.DB, "wallet_topups", id, false)
		if err != nil {
			return err
		}
		return a.applyWalletTopup(ctx, p, v, false)
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	if err = tx.QueryRow(ctx, "SELECT id FROM orders WHERE data->>'order_no'=$1", v.OrderNo).Scan(&id); err != nil {
		return fail(404, "支付订单不存在")
	}
	o, err := lockWalletOrder(ctx, tx, id, "")
	if err != nil {
		return err
	}
	c, err = a.loadWXPayForApp(o.PaymentAppID)
	if err != nil {
		return err
	}
	if err = requireLiveOrder(o); err != nil {
		return err
	}
	if baseOrderAmount(o) != v.Amount.Total || o.PayOpenID == "" || o.PayOpenID != v.Payer.OpenID {
		return fail(400, "支付金额或付款人不匹配")
	}
	if o.PaymentMerchantID != "" && (o.PaymentMerchantID != v.MerchantID || o.PaymentAppID != v.AppID) {
		return fail(409, "官方回调与已锁定原支付应用或商户不一致")
	}
	if o.PaymentTransactionID != "" {
		if o.PaymentTransactionID == v.TransactionID {
			return nil
		}
		return fail(409, "订单支付流水冲突")
	}
	if o.Status != "pending_payment" || o.PaymentStatus != "unpaid" {
		return fail(409, "订单支付状态不匹配，请人工核查")
	}
	o.PaymentTransactionID = v.TransactionID
	o.PaymentMerchantID = v.MerchantID
	o.PaymentAppID = v.AppID
	if err = accountWalletPayment(ctx, tx, &o, "wechat", ""); err != nil {
		return err
	}
	if v.SuccessTime != "" {
		stamp, e := time.Parse(time.RFC3339Nano, v.SuccessTime)
		if e != nil {
			return fail(400, "微信付款成功时间无效")
		}
		o.PaidAt = stamp.UTC().Format(time.RFC3339Nano)
	}
	o.PaymentTransactionID = v.TransactionID
	o.PaymentStatus = "paid"
	o.Status = "open"
	o.UpdatedAt = now()
	if err = put(ctx, tx, "orders", o.ID, o); err != nil {
		return err
	}
	if err = systemMessage(ctx, tx, o, "微信付款已核验成功，等待打手接单。"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// applyWXRefund 在微信确认退款成功后关闭订单；异常退款保持争议和结算冻结。
func (a *App) applyWXRefund(ctx context.Context, c wxPayConfig, v wxRefund) error {
	if v.MerchantID != "" && v.MerchantID != c.MerchantID {
		return fail(400, "退款商户不匹配")
	}
	if strings.HasPrefix(v.OutRefundNo, "OR") {
		return a.applyOrderPartRefund(ctx, c, v, false)
	}
	status := v.RefundStatus
	if status == "" {
		status = v.Status
	}
	if !contains([]string{"SUCCESS", "ABNORMAL", "CLOSED"}, status) {
		return fail(400, "退款尚未完成")
	}
	if strings.HasPrefix(v.OutRefundNo, "DR") {
		return a.applyDepositRefund(ctx, c, v, false)
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	if err = tx.QueryRow(ctx, "SELECT id FROM orders WHERE data->>'refund_id'=$1", v.OutRefundNo).Scan(&id); err != nil {
		return fail(404, "退款订单不存在")
	}
	o, err := lockWalletOrder(ctx, tx, id, "")
	if err != nil {
		return err
	}
	c, err = a.loadWXPayForApp(o.PaymentAppID)
	if err != nil {
		return err
	}
	if err = requireLiveOrder(o); err != nil {
		return err
	}
	if o.OrderNo != v.OrderNo || o.RefundID != v.OutRefundNo || o.PaymentTransactionID == "" || o.PaymentTransactionID != v.TransactionID || v.Amount.Total != o.AmountCents || v.Amount.Refund != o.AmountCents || (v.Amount.Currency != "" && v.Amount.Currency != "CNY") {
		return fail(400, "退款金额或流水不匹配")
	}
	if o.PaymentStatus == "refunded" {
		return nil
	}
	if o.PaymentStatus != "paid" {
		return fail(409, "原支付尚未确认")
	}
	message := "微信已确认全额退款，交易关闭。"
	if status == "SUCCESS" {
		if err = refundWalletEscrow(ctx, tx, &o, true); err != nil {
			return err
		}
		if v.SuccessTime != "" {
			stamp, e := time.Parse(time.RFC3339Nano, v.SuccessTime)
			if e != nil {
				return fail(400, "微信退款成功时间无效")
			}
			o.RefundedAt = stamp.UTC().Format(time.RFC3339Nano)
		}
		o.Status = "refunded"
		o.PaymentStatus = "refunded"
		o.RefundStatus = "approved"
	} else {
		reply := "微信退款异常（" + status + "），资金仍冻结，请平台到商户侧核查后同步状态。"
		if o.RefundStatus == "processing" && o.RefundReply == reply {
			return nil
		}
		o.RefundStatus = "processing"
		o.RefundReply = reply
		message = reply
	}
	o.UpdatedAt = now()
	if err = put(ctx, tx, "orders", o.ID, o); err != nil {
		return err
	}
	if err = systemMessage(ctx, tx, o, message); err != nil {
		return err
	}
	if err = refreshServiceStats(ctx, tx, o.ServiceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// syncPayment 用微信查询接口补偿回调延迟，查询结果同样必须验签与金额校验。
func (a *App) syncPayment(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	o, err := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if err = requireLiveOrder(o); err != nil {
		return nil, err
	}
	if u.ID != o.BuyerID && u.Role != "admin" {
		return nil, fail(403, "仅买家或平台可同步资金状态")
	}
	if o.AdjustmentVersion > 0 && o.RefundStatus == "processing" {
		if err = a.processOrderRefunds(r.Context(), o.ID, "full:"+o.ID); err != nil {
			return nil, err
		}
		return get[Order](r.Context(), a.DB, "orders", o.ID, false)
	}
	if o.PaymentChannel == "virtual" {
		p, e := get[virtualConsumption](r.Context(), a.DB, "virtual_consumptions", virtualConsumptionID(o.ID, ""), false)
		if e != nil {
			return nil, e
		}
		if err = a.syncVirtualConsumption(r.Context(), p); err != nil {
			return nil, err
		}
		return get[Order](r.Context(), a.DB, "orders", o.ID, false)
	}
	if a.Config.DevMode || o.PaymentChannel == "balance" {
		return o, nil
	}
	c, err := a.loadWXPayForApp(o.PaymentAppID)
	if err != nil {
		return nil, err
	}
	if o.RefundStatus == "processing" {
		var result wxRefund
		if err = a.wxRequest(r.Context(), c, "GET", "/v3/refund/domestic/refunds/"+url.PathEscape(o.RefundID), nil, &result); err != nil {
			return nil, err
		}
		if result.Status != "PROCESSING" {
			err = a.applyWXRefund(r.Context(), c, result)
		}
	} else if o.Status == "pending_payment" {
		var result wxTransaction
		if err = a.wxRequest(r.Context(), c, "GET", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(o.OrderNo)+"?mchid="+url.QueryEscape(c.MerchantID), nil, &result); err != nil {
			return nil, err
		}
		if result.TradeState == "SUCCESS" {
			err = a.applyWXPayment(r.Context(), c, result)
		}
	}
	if err != nil {
		return nil, err
	}
	return get[Order](r.Context(), a.DB, "orders", o.ID, false)
}

// retryRefund 允许管理员对未知结果重试同一退款单号，微信幂等保证不会重复退款。
func (a *App) retryRefund(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	o, err := get[Order](r.Context(), a.DB, "orders", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if err = requireLiveOrder(o); err != nil {
		return nil, err
	}
	if o.AdjustmentVersion > 0 && o.RefundStatus == "processing" {
		if err = a.processOrderRefunds(r.Context(), o.ID, "full:"+o.ID); err != nil {
			return nil, err
		}
		return get[Order](r.Context(), a.DB, "orders", o.ID, false)
	}
	if a.Config.DevMode || o.RefundStatus != "processing" || o.RefundID == "" || o.PaymentStatus != "paid" {
		return nil, fail(409, "当前订单没有待重试的真实退款")
	}
	complete, err := a.refund(r.Context(), o, o.RefundReason)
	if err != nil {
		return nil, err
	}
	if complete {
		c, configErr := a.loadWXPayForApp(o.PaymentAppID)
		if configErr != nil {
			return nil, configErr
		}
		result := wxRefund{MerchantID: c.MerchantID, OrderNo: o.OrderNo, OutRefundNo: o.RefundID, TransactionID: o.PaymentTransactionID, Status: "SUCCESS"}
		result.Amount.Total = o.AmountCents
		result.Amount.Refund = o.AmountCents
		if err = a.applyWXRefund(r.Context(), c, result); err != nil {
			return nil, err
		}
	}
	if err = audit(r.Context(), a.DB, u.ID, "order.refund.retry", o.ID, "使用既有退款单号重试，未创建第二笔退款"); err != nil {
		return nil, err
	}
	return get[Order](r.Context(), a.DB, "orders", o.ID, false)
}
