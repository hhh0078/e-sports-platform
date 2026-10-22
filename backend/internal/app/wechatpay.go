// Package app 接入微信支付 APIv3 的签名请求、应答验签、预支付、退款与关单。
package app

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type wxPayConfig struct {
	AppID, MerchantID, Serial, PublicKeyID, APIKey, NotifyURL, RefundNotifyURL string
	PrivateKey                                                                 *rsa.PrivateKey
	PublicKey                                                                  *rsa.PublicKey
}

// wxAPIError 仅承载已经平台验签的错误码，供关单安全地区分未建单与未知状态。
type wxAPIError struct{ Code string }

// Error 返回统一提示，避免把商户配置和微信内部信息泄露给客户端。
func (e *wxAPIError) Error() string {
	return "微信支付未受理当前操作，请同步交易状态或检查商户配置"
}

// Unwrap 将外部渠道错误映射为网关错误，保留统一接口错误格式。
func (e *wxAPIError) Unwrap() error { return fail(502, e.Error()) }

// loadWXPay 读取已加密落库的只读快照，未使用管理配置时兼容环境变量部署。
func (a *App) loadWXPay() (wxPayConfig, error) {
	a.paymentConfigMu.RLock()
	snapshot := a.paymentConfig
	a.paymentConfigMu.RUnlock()
	if snapshot != nil {
		if !snapshot.Ready {
			return wxPayConfig{}, fail(503, "微信支付配置不完整，未产生扣款")
		}
		return snapshot.Runtime, nil
	}
	return a.runtimePaymentConfig(paymentSettingsFromEnvironment())
}

// validNotifyURL 要求生产回调使用 HTTPS 地址且不携带账号密码或片段。
func validNotifyURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}

// wxSign 使用商户私钥对微信规定的待签名串签名。
func wxSign(key *rsa.PrivateKey, message string) (string, error) {
	sum := sha256.Sum256([]byte(message))
	signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	return base64.StdEncoding.EncodeToString(signature), e
}

// wxVerify 校验时间、序列号和 RSA 签名，拒绝伪造、篡改和过期通知。
func wxVerify(c wxPayConfig, header http.Header, body []byte) error {
	stamp := header.Get("Wechatpay-Timestamp")
	seconds, e := strconv.ParseInt(stamp, 10, 64)
	if e != nil || time.Since(time.Unix(seconds, 0)) > 5*time.Minute || time.Until(time.Unix(seconds, 0)) > 5*time.Minute {
		return fail(401, "微信支付消息时间无效")
	}
	nonce := header.Get("Wechatpay-Nonce")
	if nonce == "" || header.Get("Wechatpay-Serial") != c.PublicKeyID {
		return fail(401, "微信支付验签标识无效")
	}
	signature, e := base64.StdEncoding.DecodeString(header.Get("Wechatpay-Signature"))
	if e != nil {
		return fail(401, "微信支付签名无效")
	}
	sum := sha256.Sum256([]byte(stamp + "\n" + nonce + "\n" + string(body) + "\n"))
	if rsa.VerifyPKCS1v15(c.PublicKey, crypto.SHA256, sum[:], signature) != nil {
		return fail(401, "微信支付签名验证失败")
	}
	return nil
}

// wxRequest 调用固定微信商户域名，对成功响应验签后再解析业务内容。
func (a *App) wxRequest(ctx context.Context, c wxPayConfig, method, path string, input, out any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := newID()
	signature, err := wxSign(c.PrivateKey, method+"\n"+path+"\n"+stamp+"\n"+nonce+"\n"+string(body)+"\n")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.mch.weixin.qq.com"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`, c.MerchantID, nonce, stamp, c.Serial, signature))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Wechatpay-Serial", c.PublicKeyID)
	response, err := a.HTTPClient.Do(req)
	if err != nil {
		return fail(502, "微信支付暂未响应，请稍后同步订单状态，勿重复付款")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 {
		return fail(502, "微信支付响应异常")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// 只有认证过的业务错误能改变交易判断；代理404、无签名响应仍视为未知结果。
		if wxVerify(c, response.Header, raw) == nil {
			var failure struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(raw, &failure) == nil && failure.Code != "" {
				return &wxAPIError{Code: failure.Code}
			}
		}
		return fail(502, "微信支付未受理当前操作，请同步交易状态或检查商户配置")
	}
	if err = wxVerify(c, response.Header, raw); err != nil {
		return fail(502, "微信支付应答验签失败，订单未确认")
	}
	if out != nil && len(raw) > 0 {
		if err = json.Unmarshal(raw, out); err != nil {
			return fail(502, "微信支付返回格式无效")
		}
	}
	return nil
}

// prepay 创建微信小程序预支付会话，仅返回拉起支付参数，不把客户端操作视为付款成功。
func (a *App) prepay(r *http.Request, u User, o Order) (any, error) {
	if err := requireLiveOrder(o); err != nil {
		return nil, err
	}
	c, err := a.loadWXPay()
	if err != nil {
		return nil, err
	}
	if o.PayOpenID == "" {
		return nil, fail(409, "请先用微信登录后再付款")
	}
	payload := map[string]any{"appid": c.AppID, "mchid": c.MerchantID, "description": o.Title, "out_trade_no": o.OrderNo, "notify_url": c.NotifyURL, "amount": map[string]any{"total": o.AmountCents, "currency": "CNY"}, "payer": map[string]string{"openid": o.PayOpenID}}
	var result struct {
		PrepayID string `json:"prepay_id"`
	}
	if err = a.wxRequest(r.Context(), c, "POST", "/v3/pay/transactions/jsapi", payload, &result); err != nil {
		return nil, err
	}
	if result.PrepayID == "" {
		return nil, fail(502, "微信未返回有效支付会话")
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := newID()
	pack := "prepay_id=" + result.PrepayID
	signature, err := wxSign(c.PrivateKey, c.AppID+"\n"+stamp+"\n"+nonce+"\n"+pack+"\n")
	if err != nil {
		return nil, err
	}
	return map[string]any{"order": o, "mode": "wechat", "pay_params": map[string]string{"timeStamp": stamp, "nonceStr": nonce, "package": pack, "signType": "RSA", "paySign": signature}}, nil
}

// refund 用稳定退款单号申请全额退款，成功资源经幂等入账并保留官方时间后才返回 true。
func (a *App) refund(ctx context.Context, o Order, reason string) (bool, error) {
	if err := requireLiveOrder(o); err != nil {
		return false, err
	}
	c, err := a.loadWXPay()
	if err != nil {
		return false, err
	}
	if o.PaymentTransactionID == "" {
		return false, fail(409, "缺少真实支付流水，不能执行真实退款")
	}
	if o.RefundID == "" {
		o.RefundID = "RF" + o.ID
	}
	runes := []rune(strings.TrimSpace(reason))
	if len(runes) > 26 {
		reason = string(runes[:26])
	}
	payload := map[string]any{"transaction_id": o.PaymentTransactionID, "out_refund_no": o.RefundID, "reason": reason, "notify_url": c.RefundNotifyURL, "amount": map[string]any{"refund": o.AmountCents, "total": o.AmountCents, "currency": "CNY"}}
	var result wxRefund
	if err = a.wxRequest(ctx, c, "POST", "/v3/refund/domestic/refunds", payload, &result); err != nil {
		return false, err
	}
	if result.OutRefundNo != o.RefundID || result.TransactionID != o.PaymentTransactionID || result.Amount.Total != o.AmountCents || result.Amount.Refund != o.AmountCents {
		return false, fail(502, "微信退款金额或流水校验失败")
	}
	if result.Status == "SUCCESS" {
		// 原申请已经事务提交；直接使用已验签资源入账，保留官方成功时间，后续重复调用仍幂等。
		if result.OrderNo == "" {
			result.OrderNo = o.OrderNo
		}
		if result.MerchantID == "" {
			result.MerchantID = c.MerchantID
		}
		if err = a.applyWXRefund(ctx, c, result); err != nil {
			return false, err
		}
		return true, nil
	}
	if result.Status == "PROCESSING" {
		return false, nil
	}
	return false, fail(502, "微信退款未完成，请由管理员核查")
}

// closePayment 先关闭微信预支付单再允许本地取消，避免取消后仍发生实际扣款。
func (a *App) closePayment(ctx context.Context, o Order) error {
	if err := requireLiveOrder(o); err != nil {
		return err
	}
	c, err := a.loadWXPay()
	if err != nil {
		return err
	}
	err = a.wxRequest(ctx, c, "POST", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(o.OrderNo)+"/close", map[string]string{"mchid": c.MerchantID}, nil)
	var businessError *wxAPIError
	if errors.As(err, &businessError) && businessError.Code == "ORDER_NOT_EXIST" {
		return nil
	}
	return err
}
