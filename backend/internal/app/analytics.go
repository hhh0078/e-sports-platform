// Package app 按上海自然日统计有真实资金来源与可靠发生时间的服务交易，充值、押金及提现不进入成交额。
package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	_ "time/tzdata"
)

// AnalyticsPeriod 明确返回闭区间日期和自然日时区，前端无需推测UTC偏移。
type AnalyticsPeriod struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// AnalyticsSummary 汇总原始付款与成功退款，退款不会倒扣历史付款日的成交额。
type AnalyticsSummary struct {
	GMVCents           int64 `json:"gmv_cents"`
	RefundCents        int64 `json:"refund_cents"`
	PaidOrderCount     int64 `json:"paid_order_count"`
	RefundedOrderCount int64 `json:"refunded_order_count"`
}

// AnalyticsDay 即使当日没有资金事件也返回零值，确保曲线不跳过自然日。
type AnalyticsDay struct {
	Date           string `json:"date"`
	PaidCents      int64  `json:"paid_cents"`
	RefundCents    int64  `json:"refund_cents"`
	PaidOrders     int64  `json:"paid_orders"`
	RefundedOrders int64  `json:"refunded_orders"`
}

// AnalyticsResult 只包含汇总数值，不返回用户身份、支付流水或商户配置。
type AnalyticsResult struct {
	Period                AnalyticsPeriod  `json:"period"`
	Summary               AnalyticsSummary `json:"summary"`
	Series                []AnalyticsDay   `json:"series"`
	LegacyUntrackedOrders int64            `json:"legacy_untracked_orders"`
	DevMode               bool             `json:"dev_mode"`
	DataMode              string           `json:"data_mode"`
	StatisticsNote        string           `json:"statistics_note"`
}

// analyticsDates 校验成对日期参数，按上海自然日计算闭区间，包含闰日且最多366日。
func analyticsDates(r *http.Request, stamp time.Time) (AnalyticsPeriod, []AnalyticsDay, *time.Location, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return AnalyticsPeriod{}, nil, nil, err
	}
	query := r.URL.Query()
	startRaw, endRaw := query.Get("start"), query.Get("end")
	_, startSet := query["start"]
	_, endSet := query["end"]
	if !startSet && !endSet {
		endRaw = stamp.In(location).Format("2006-01-02")
		today, _ := time.ParseInLocation("2006-01-02", endRaw, location)
		startRaw = today.AddDate(0, 0, -6).Format("2006-01-02")
	}
	if len(startRaw) != 10 || len(endRaw) != 10 {
		return AnalyticsPeriod{}, nil, nil, fail(400, "起止日期需同时填写，格式为YYYY-MM-DD")
	}
	start, err := time.ParseInLocation("2006-01-02", startRaw, location)
	if err != nil {
		return AnalyticsPeriod{}, nil, nil, fail(400, "开始日期无效")
	}
	end, err := time.ParseInLocation("2006-01-02", endRaw, location)
	if err != nil || end.Before(start) {
		return AnalyticsPeriod{}, nil, nil, fail(400, "结束日期无效或早于开始日期")
	}
	days := []AnalyticsDay{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		if len(days) >= 366 {
			return AnalyticsPeriod{}, nil, nil, fail(400, "统计周期最多366个自然日")
		}
		days = append(days, AnalyticsDay{Date: day.Format("2006-01-02")})
	}
	return AnalyticsPeriod{Start: startRaw, End: endRaw, Timezone: "Asia/Shanghai"}, days, location, nil
}

// analyticsPaymentEvidence 要求订单与逐单实付款托管一致，伪造完成/付款状态或无资金凭据均不可计成交额。
func analyticsPaymentEvidence(o Order, e WalletEscrow, dev bool) bool {
	if !o.WalletAccounted || o.AmountCents <= 0 || o.AmountCents != e.AmountCents || o.ID != e.OrderID || o.BuyerID != e.BuyerID || o.ProviderID != e.ProviderID || o.PaymentChannel != e.Channel || !contains([]string{"paid", "refunded"}, o.PaymentStatus) || !contains([]string{"holding", "released", "refunded"}, e.Status) {
		return false
	}
	switch e.Channel {
	case "wechat":
		return o.PaymentTransactionID != "" && !strings.HasPrefix(o.PaymentTransactionID, "DEV")
	case "balance":
		return e.RechargeUsed >= 0 && e.IncomeUsed >= 0 && e.RechargeUsed <= o.AmountCents && e.IncomeUsed == o.AmountCents-e.RechargeUsed
	case "dev":
		return dev
	default:
		return false
	}
}

// analytics 仅平台管理员可读取；逐行校验可解析的发生时间，绝不用created_at或updated_at代替支付/退款时间。
func (a *App) analytics(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	period, days, location, err := analyticsDates(r, time.Now())
	if err != nil {
		return nil, err
	}
	out := AnalyticsResult{Period: period, Series: days, DevMode: a.Config.DevMode, DataMode: "production", StatisticsNote: "服务成交额按首次实付记录统计，成功退款按退款日另计；微信提供成功时间时采用官方时间，否则采用平台首次核验入账时间。充值、押金和提现不计服务成交额。历史缺少可靠支付/退款时间或托管资金凭据的订单不计对应曲线，数量为全历史待核账订单数。"}
	if a.Config.DevMode {
		out.DataMode = "local"
		out.StatisticsNote = "当前为本地模拟交易统计，不代表真实资金。" + out.StatisticsNote
	}
	indices := map[string]int{}
	for i, day := range days {
		indices[day.Date] = i
	}
	// 仅有明确已付/已退标记的业务单参与核查；再以实付托管核验来源，状态本身不能成为资金凭据。
	rows, err := a.DB.Query(r.Context(), `SELECT o.data,e.data FROM orders o LEFT JOIN wallet_escrows e ON e.id=o.id
 WHERE left(o.id,5)<>'demo_' AND COALESCE(o.data->>'is_demo','false')<>'true' AND o.data->>'payment_status' IN ('paid','refunded')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orderRaw, escrowRaw []byte
		if err = rows.Scan(&orderRaw, &escrowRaw); err != nil {
			return nil, err
		}
		var order Order
		var escrow WalletEscrow
		if err = json.Unmarshal(orderRaw, &order); err != nil {
			return nil, err
		}
		if len(escrowRaw) > 0 {
			if err = json.Unmarshal(escrowRaw, &escrow); err != nil {
				return nil, err
			}
		}
		if !analyticsPaymentEvidence(order, escrow, a.Config.DevMode) {
			out.LegacyUntrackedOrders++
			continue
		}
		untracked := false
		paid, paidErr := time.Parse(time.RFC3339Nano, order.PaidAt)
		if paidErr != nil {
			untracked = true
		} else if index, ok := indices[paid.In(location).Format("2006-01-02")]; ok {
			out.Series[index].PaidCents += order.AmountCents
			out.Series[index].PaidOrders++
			out.Summary.GMVCents += order.AmountCents
			out.Summary.PaidOrderCount++
		}
		if order.PaymentStatus == "refunded" {
			refunded, refundErr := time.Parse(time.RFC3339Nano, order.RefundedAt)
			if refundErr != nil || escrow.Status != "refunded" {
				untracked = true
			} else if index, ok := indices[refunded.In(location).Format("2006-01-02")]; ok {
				out.Series[index].RefundCents += order.AmountCents
				out.Series[index].RefundedOrders++
				out.Summary.RefundCents += order.AmountCents
				out.Summary.RefundedOrderCount++
			}
		}
		if untracked {
			out.LegacyUntrackedOrders++
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
