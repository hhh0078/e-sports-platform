// Package app 保存订单调价协商、独立补款和逐笔原路退款，原支付事实不随净价变化。
package app

import (
	"context"
	"net/http"
)

// OrderAdjustment 保存双方明确确认的差额方案；不包含微信身份、签名或支付密钥。
type OrderAdjustment struct {
	PaymentChannel       string `json:"payment_channel,omitempty"`
	ID                   string `json:"id"`
	OrderID              string `json:"order_id"`
	Kind                 string `json:"kind"`
	AmountCents          int64  `json:"amount_cents"`
	Reason               string `json:"reason"`
	ProposerID           string `json:"proposer_id"`
	ProposerName         string `json:"proposer_name"`
	Status               string `json:"status"`
	BuyerAgreed          bool   `json:"buyer_agreed"`
	ProviderAgreed       bool   `json:"provider_agreed"`
	ExpiresAt            string `json:"expires_at"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
	CompletedAt          string `json:"completed_at,omitempty"`
	FailureReason        string `json:"failure_reason,omitempty"`
	PaymentCreatingUntil string `json:"payment_creating_until,omitempty"`
	RequestID            string `json:"request_id,omitempty"`
}

// OrderPayment 是每笔订单实付的稳定原单；只有退款已用及预留容量能够在付款成功后改变。
type OrderPayment struct {
	ID            string `json:"id"`
	OrderID       string `json:"order_id"`
	AdjustmentID  string `json:"adjustment_id"`
	BuyerID       string `json:"buyer_id"`
	Kind          string `json:"kind"`
	Channel       string `json:"channel"`
	AmountCents   int64  `json:"amount_cents"`
	Status        string `json:"status"`
	OrderNo       string `json:"order_no"`
	TransactionID string `json:"transaction_id"`
	MerchantID    string `json:"merchant_id"`
	AppID         string `json:"app_id"`
	PayOpenID     string `json:"pay_open_id"`
	RequestID     string `json:"request_id"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	RechargeUsed  int64  `json:"recharge_used_cents"`
	IncomeUsed    int64  `json:"income_used_cents"`
	RefundedCents int64  `json:"refunded_cents"`
	ReservedCents int64  `json:"reserved_cents"`
	CreatedAt     string `json:"created_at"`
	PaidAt        string `json:"paid_at"`
	UpdatedAt     string `json:"updated_at"`
}

// OrderPartRefund 冻结某笔付款的精确资金批次，未知外部结果仍沿用原退款号。
type OrderPartRefund struct {
	Purpose      string        `json:"purpose,omitempty"`
	ID           string        `json:"id"`
	OrderID      string        `json:"order_id"`
	AdjustmentID string        `json:"adjustment_id"`
	GroupID      string        `json:"group_id"`
	PaymentID    string        `json:"payment_id"`
	RefundNo     string        `json:"refund_no"`
	AmountCents  int64         `json:"amount_cents"`
	Status       string        `json:"status"`
	Reason       string        `json:"reason"`
	Sources      []FundingPart `json:"sources"`
	SuccessAt    string        `json:"success_at"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

// registerAdjustmentRoutes 注册调价查询、双方回应及独立补款和补偿接口。
func (a *App) registerAdjustmentRoutes(mux *http.ServeMux) {
	for path, fn := range map[string]func(http.ResponseWriter, *http.Request) (any, error){
		"GET /api/orders/{id}/adjustments":                          a.orderAdjustments,
		"POST /api/orders/{id}/adjustments":                         a.proposeOrderAdjustment,
		"POST /api/orders/{id}/adjustments/{adjustment_id}/respond": a.respondOrderAdjustment,
		"POST /api/orders/{id}/adjustments/{adjustment_id}/pay":     a.payOrderAdjustment,
		"POST /api/orders/{id}/adjustments/{adjustment_id}/sync":    a.syncOrderAdjustment,
	} {
		mux.HandleFunc(path, a.wrap(fn))
	}
}

// migrateOrderAdjustments 增加唯一业务号与原单事实保护，不回填或伪造历史收款。
func migrateOrderAdjustments(ctx context.Context, q querier) error {
	_, e := q.Exec(ctx, `
-- 每个调价请求只产生一个方案，每个方案只产生一笔补款原单。
CREATE UNIQUE INDEX IF NOT EXISTS order_adjustments_request_unique ON order_adjustments ((data->>'proposer_id'),(data->>'request_id'));
CREATE INDEX IF NOT EXISTS order_adjustments_order_idx ON order_adjustments ((data->>'order_id'),(data->>'created_at'));
CREATE UNIQUE INDEX IF NOT EXISTS order_payments_number_unique ON order_payments ((data->>'order_no'));
CREATE UNIQUE INDEX IF NOT EXISTS order_payments_adjustment_unique ON order_payments ((data->>'adjustment_id')) WHERE COALESCE(data->>'adjustment_id','')<>'';
CREATE UNIQUE INDEX IF NOT EXISTS order_payments_transaction_unique ON order_payments ((data->>'transaction_id')) WHERE COALESCE(data->>'transaction_id','')<>'';
CREATE INDEX IF NOT EXISTS order_payments_order_idx ON order_payments ((data->>'order_id'));
CREATE UNIQUE INDEX IF NOT EXISTS order_refunds_number_unique ON order_refunds ((data->>'refund_no'));
CREATE INDEX IF NOT EXISTS order_refunds_group_idx ON order_refunds ((data->>'order_id'),(data->>'group_id'));
-- 成功付款的原价、支付身份与来源不能被调价或退款改写。
CREATE OR REPLACE FUNCTION protect_order_payment_origin() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
IF OLD.data->>'status'='paid' AND (OLD.data-'refunded_cents'-'reserved_cents'-'updated_at') IS DISTINCT FROM (NEW.data-'refunded_cents'-'reserved_cents'-'updated_at') THEN RAISE EXCEPTION '订单原付款事实不可改写'; END IF; RETURN NEW;END $$;
DROP TRIGGER IF EXISTS order_payment_origin_immutable ON order_payments;
CREATE TRIGGER order_payment_origin_immutable BEFORE UPDATE ON order_payments FOR EACH ROW EXECUTE FUNCTION protect_order_payment_origin();
DROP TRIGGER IF EXISTS order_payment_delete_immutable ON order_payments;
CREATE TRIGGER order_payment_delete_immutable BEFORE DELETE ON order_payments FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
COMMENT ON FUNCTION protect_order_payment_origin() IS '成功原付款只允许更新退款已用和预留容量';
CREATE OR REPLACE FUNCTION protect_order_refund_origin() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF (OLD.data-'status'-'success_at'-'updated_at') IS DISTINCT FROM (NEW.data-'status'-'success_at'-'updated_at') THEN RAISE EXCEPTION '订单退款原单与来源不可改写';END IF;RETURN NEW;END $$;
DROP TRIGGER IF EXISTS order_refund_origin_immutable ON order_refunds;
CREATE TRIGGER order_refund_origin_immutable BEFORE UPDATE ON order_refunds FOR EACH ROW EXECUTE FUNCTION protect_order_refund_origin();
DROP TRIGGER IF EXISTS order_refund_delete_immutable ON order_refunds;
CREATE TRIGGER order_refund_delete_immutable BEFORE DELETE ON order_refunds FOR EACH ROW EXECUTE FUNCTION reject_wallet_entry_mutation();
COMMENT ON FUNCTION protect_order_refund_origin() IS '订单退款原交易、金额、来源批次与业务归属不可修改';
CREATE OR REPLACE FUNCTION protect_adjusted_order_origin() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
IF COALESCE((OLD.data->>'adjustment_version')::int,0)>0 AND (OLD.data->'original_amount_cents' IS DISTINCT FROM NEW.data->'original_amount_cents' OR OLD.data->'payment_transaction_id' IS DISTINCT FROM NEW.data->'payment_transaction_id' OR OLD.data->'payment_channel' IS DISTINCT FROM NEW.data->'payment_channel' OR OLD.data->'payment_merchant_id' IS DISTINCT FROM NEW.data->'payment_merchant_id' OR OLD.data->'payment_app_id' IS DISTINCT FROM NEW.data->'payment_app_id' OR OLD.data->'pay_open_id' IS DISTINCT FROM NEW.data->'pay_open_id' OR OLD.data->'paid_at' IS DISTINCT FROM NEW.data->'paid_at' OR OLD.data->'buyer_id' IS DISTINCT FROM NEW.data->'buyer_id' OR OLD.data->'provider_id' IS DISTINCT FROM NEW.data->'provider_id' OR OLD.data->'club_id' IS DISTINCT FROM NEW.data->'club_id' OR OLD.data#>'{settlement,platform_rate_bps}' IS DISTINCT FROM NEW.data#>'{settlement,platform_rate_bps}' OR OLD.data#>'{settlement,club_rate_bps}' IS DISTINCT FROM NEW.data#>'{settlement,club_rate_bps}' OR OLD.data->'adjustment_version' IS DISTINCT FROM NEW.data->'adjustment_version') THEN RAISE EXCEPTION '调价订单原价、原付款和冻结比例不可改写';END IF;RETURN NEW;END $$;
DROP TRIGGER IF EXISTS adjusted_order_origin_immutable ON orders;
CREATE TRIGGER adjusted_order_origin_immutable BEFORE UPDATE ON orders FOR EACH ROW EXECUTE FUNCTION protect_adjusted_order_origin();
COMMENT ON FUNCTION protect_adjusted_order_origin() IS '首次登记调价后固定原价、原支付身份、三方归属与原分账比例';
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='order_payment_refund_capacity' AND conrelid='order_payments'::regclass) THEN ALTER TABLE order_payments ADD CONSTRAINT order_payment_refund_capacity CHECK((data->>'amount_cents')::bigint>0 AND (data->>'refunded_cents')::bigint>=0 AND (data->>'reserved_cents')::bigint>=0 AND (data->>'refunded_cents')::bigint+(data->>'reserved_cents')::bigint<=(data->>'amount_cents')::bigint);END IF;END $$;
`)
	return e
}
