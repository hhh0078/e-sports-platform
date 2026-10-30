// Package app 对微信代币充值、余额查询与扣币退款使用官方签名协议，禁止普通支付降级和客户端成功即入账。
package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// virtualQuote 将服务净价、买家额外服务费和需购买的金币面额分开，所有金额均使用整数分。
type virtualQuote struct {
	ServiceCents     int64  `json:"service_cents"`
	FeeCents         int64  `json:"fee_cents"`
	TotalCents       int64  `json:"total_cents"`
	TopupCents       int64  `json:"topup_cents"`
	CoinBalanceCents int64  `json:"coin_balance_cents"`
	FeeBearer        string `json:"fee_bearer"`
	FeeBPS           int64  `json:"fee_bps"`
	Platform         string `json:"platform"`
}

// virtualConsumption 持久化一次官方扣币的原始归属、收费快照和未知结果，重试始终使用同一单号。
type virtualConsumption struct {
	ID              string       `json:"id"`
	OrderID         string       `json:"order_id"`
	AdjustmentID    string       `json:"adjustment_id"`
	UserID          string       `json:"user_id"`
	AppID           string       `json:"app_id"`
	OpenID          string       `json:"openid"`
	OfferID         string       `json:"offer_id"`
	Env             int          `json:"env"`
	OrderNo         string       `json:"order_no"`
	UserIP          string       `json:"user_ip"`
	Quote           virtualQuote `json:"quote"`
	TopupID         string       `json:"topup_id"`
	Status          string       `json:"status"`
	CreatedAt       string       `json:"created_at"`
	PaidAt          string       `json:"paid_at"`
	RequestPlatform string       `json:"request_platform"`
}

// virtualTransaction 保留已经向官方查询的现金充值事实和异常标记，不把金币扣减伪装为现金实收。
type virtualTransaction struct {
	ID                     string           `json:"id"`
	TopupID                string           `json:"topup_id"`
	Order                  virtualCashOrder `json:"order"`
	ReconciliationRequired bool             `json:"reconciliation_required"`
	CashRefundCents        int64            `json:"cash_refund_cents"`
	NotifiedRefundCents    int64            `json:"notified_refund_cents"`
	RecoveredCoinCents     int64            `json:"recovered_coin_cents"`
	CheckedAt              string           `json:"checked_at"`
}

// virtualCashOrder 只接收微信查单文档明确提供的字段，用原单、金额和订单类型核验现金充值。
type virtualCashOrder struct {
	OrderID      string `json:"order_id"`
	Status       int    `json:"status"`
	OrderFee     int64  `json:"order_fee"`
	PaidFee      int64  `json:"paid_fee"`
	OrderType    int    `json:"order_type"`
	PaidTime     int64  `json:"paid_time"`
	WXOrderID    string `json:"wx_order_id"`
	WXPayOrderID string `json:"wxpay_order_id"`
	LeftFee      int64  `json:"left_fee"`
	SettState    int    `json:"sett_state"`
	PlatformFee  int64  `json:"platform_fee_fen"`
}

// virtualHMAC 按官方规则输出小写十六进制HMAC，不记录密钥、用户会话或签名原串。
func virtualHMAC(key, message string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// virtualConfig 为每个原单固定应用、OfferID和环境，配置变化时关闭操作而非切换普通支付。
func (a *App) virtualConfig(appID, offerID string, env int) (commerceSettings, string, error) {
	c := a.commerceSettings()
	key := c.AppKey
	if env == 1 {
		key = c.SandboxAppKey
	}
	if appID != a.Config.WechatAppID || c.OfferID == "" || c.OfferID != offerID || env != c.VirtualEnv || key == "" || env < 0 || env > 1 {
		return c, "", fail(503, "虚拟支付配置缺失或与原单不一致，请平台核查，未改用普通支付")
	}
	return c, key, nil
}

// virtualRequest 使用固定官方接口、原始请求字节签名和禁止重定向的客户端，返回官方业务错误码。
func (a *App) virtualRequest(ctx context.Context, appID, offerID string, env int, path string, input any, sessionKey string, out any) (int, error) {
	return a.virtualRequestOnce(ctx, appID, offerID, env, path, input, sessionKey, out, false)
}

// virtualRequestOnce 只对已知凭证失效刷新一次access_token，扣币和退款仍使用完全相同的稳定原单号。
func (a *App) virtualRequestOnce(ctx context.Context, appID, offerID string, env int, path string, input any, sessionKey string, out any, refresh bool) (int, error) {
	_, key, err := a.virtualConfig(appID, offerID, env)
	if err != nil {
		return 0, err
	}
	if !contains([]string{"/xpay/query_user_balance", "/xpay/query_order", "/xpay/currency_pay", "/xpay/cancel_currency_pay"}, path) {
		return 0, fail(400, "不支持的虚拟支付操作")
	}
	if a.HTTPClient == nil {
		return 0, fail(503, "虚拟支付网络服务不可用")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	token, err := a.wechatAccessToken(ctx, refresh)
	if err != nil {
		return 0, err
	}
	query := url.Values{"access_token": {token}, "pay_sig": {virtualHMAC(key, path+"&"+string(raw))}}
	if sessionKey != "" {
		query.Set("signature", virtualHMAC(sessionKey, string(raw)))
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, "https://api.weixin.qq.com"+path+"?"+query.Encode(), bytes.NewReader(raw))
	if err != nil {
		return 0, fail(502, "虚拟支付请求无效")
	}
	req.Header.Set("Content-Type", "application/json")
	client := *a.HTTPClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return 0, fail(502, "虚拟支付结果待确认，请同步原单，勿重复购买金币")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 || response.StatusCode != http.StatusOK {
		return 0, fail(502, "虚拟支付响应未知，请保留原单继续同步")
	}
	var result struct {
		ErrCode *int `json:"errcode"`
	}
	if json.Unmarshal(body, &result) != nil || result.ErrCode == nil {
		return 0, fail(502, "虚拟支付响应格式无效，原资金状态保留")
	}
	if *result.ErrCode != 0 {
		if !refresh && (*result.ErrCode == 40001 || *result.ErrCode == 40014 || *result.ErrCode == 42001) {
			response.Body.Close()
			return a.virtualRequestOnce(ctx, appID, offerID, env, path, input, sessionKey, out, true)
		}
		return *result.ErrCode, nil
	}
	if out != nil && json.Unmarshal(body, out) != nil {
		return 0, fail(502, "虚拟支付返回数据无效")
	}
	return 0, nil
}

// virtualResultError 只展示可行动的安全错误，不泄露微信原始请求或支付秘密。
func virtualResultError(code int) error {
	if code == 0 || code == 268490004 {
		return nil
	}
	if code == 268490009 {
		return fail(409, "微信支付会话已过期，请刷新登录会话后同步原单")
	}
	if code == 268490006 {
		return fail(409, "官方金币余额不足，请同步充值原单")
	}
	return fail(502, "微信虚拟支付未完成，请平台核查或继续同步原单")
}

// makeVirtualQuote 采用配置快照计算额外金币，不允许小数金币或通过舍入暗改服务价格。
func makeVirtualQuote(c commerceSettings, service, balance int64, platform string) (virtualQuote, error) {
	q := virtualQuote{ServiceCents: service, CoinBalanceCents: balance, FeeBearer: c.FeeBearer, Platform: platform}
	if service <= 0 || service > 10000000 || balance < 0 || !contains([]string{"android", "ios"}, platform) {
		return q, fail(400, "金币报价金额或终端无效")
	}
	q.FeeBPS = c.AndroidFeeBPS
	if platform == "ios" {
		q.FeeBPS = c.IOSFeeBPS
	}
	if q.FeeBPS < 0 || q.FeeBPS > 10000 {
		return q, fail(503, "平台手续费配置无效")
	}
	if c.FeeBearer == "buyer" {
		q.FeeCents = (service*q.FeeBPS + 9999) / 10000
	} else if c.FeeBearer != "platform" {
		return q, fail(503, "手续费承担配置无效")
	}
	q.TotalCents = service + q.FeeCents
	q.TopupCents = q.TotalCents - balance
	if q.TopupCents < 0 {
		q.TopupCents = 0
	}
	if service%100 != 0 || q.TotalCents%100 != 0 || q.TopupCents%100 != 0 {
		return q, fail(409, "微信1:1金币的小数精度尚未核实，当前仅支持整金币付款；未对服务价或手续费取整")
	}
	return q, nil
}

// registerVirtualPaymentRoutes 注册只读报价；实际支付继续使用有归属校验的原订单、补款和钱包接口。
func (a *App) registerVirtualPaymentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/payments/virtual/quote", a.wrap(a.virtualPaymentQuote))
	mux.HandleFunc("GET /api/wechat/virtual-payment/callback", a.virtualPaymentCallback)
	mux.HandleFunc("POST /api/wechat/virtual-payment/callback", a.virtualPaymentCallback)
}

// virtualPaymentQuote 仅为当前用户报价，客户端传入服务额不能构成实际付款授权。
func (a *App) virtualPaymentQuote(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	var in struct {
		ServiceCents int64  `json:"service_cents"`
		Platform     string `json:"platform"`
		OrderID      string `json:"order_id"`
		AdjustmentID string `json:"adjustment_id"`
	}
	if e = decode(r, &in); e != nil {
		return nil, e
	}
	if in.OrderID != "" {
		o, e := a.authorizedOrder(r, u, in.OrderID, false)
		if e != nil {
			return nil, e
		}
		if o.BuyerID != u.ID {
			return nil, fail(403, "只有买家可确认付款报价")
		}
		in.ServiceCents = o.AmountCents
		if in.AdjustmentID != "" {
			v, e := get[OrderAdjustment](r.Context(), a.DB, "order_adjustments", in.AdjustmentID, false)
			if e != nil {
				return nil, e
			}
			if v.OrderID != o.ID || v.Kind != "increase" {
				return nil, fail(409, "补款报价归属不一致")
			}
			in.ServiceCents = v.AmountCents
		}
		old, e := get[virtualConsumption](r.Context(), a.DB, "virtual_consumptions", virtualConsumptionID(o.ID, in.AdjustmentID), false)
		if e == nil {
			if old.UserID != u.ID {
				return nil, fail(403, "报价归属不一致")
			}
			return old.Quote, nil
		} else if !missing(e) {
			return nil, e
		}
	} else if in.AdjustmentID != "" {
		return nil, fail(400, "补款报价须指定原订单")
	}
	wallet, e := get[Wallet](r.Context(), a.DB, "wallets", u.ID, false)
	if missing(e) {
		wallet = Wallet{UserID: u.ID}
	} else if e != nil {
		return nil, e
	}
	c := a.commerceSettings()
	platform, e := virtualFeePlatform(r.Context(), a.DB, u.ID, c, strings.ToLower(in.Platform))
	if e != nil {
		return nil, e
	}
	return makeVirtualQuote(c, in.ServiceCents, wallet.VirtualCoinCents, platform)
}

// migrateVirtualPayments 增加虚拟资金独立非负约束及幂等索引，禁止把现金钱包当作官方金币。
func migrateVirtualPayments(ctx context.Context, q querier) error {
	_, e := q.Exec(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS virtual_consumptions_number_unique ON virtual_consumptions ((data->>'order_no'));
CREATE INDEX IF NOT EXISTS virtual_consumptions_user_idx ON virtual_consumptions ((data->>'user_id'),(data->>'status'));
CREATE UNIQUE INDEX IF NOT EXISTS virtual_transactions_topup_unique ON virtual_transactions ((data->>'topup_id'));
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='wallets_virtual_nonnegative' AND conrelid='wallets'::regclass) THEN ALTER TABLE wallets ADD CONSTRAINT wallets_virtual_nonnegative CHECK(COALESCE((data->>'virtual_coin_cents')::bigint,0)>=0); END IF; END $$;
COMMENT ON INDEX virtual_consumptions_number_unique IS '官方扣币原单号唯一，未知结果重用原单';
COMMENT ON INDEX virtual_consumptions_user_idx IS '按账号恢复未完成虚拟扣币，不跨账号记账';
COMMENT ON INDEX virtual_transactions_topup_unique IS '一个现金充值原单只保留一份官方核验记录';
`)
	return e
}
