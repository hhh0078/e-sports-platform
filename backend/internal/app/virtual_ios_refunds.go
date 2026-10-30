// Package app 仅以已核验充值与实际履约来源向Apple提供退款建议，证据不足时不伪造拒绝理由。
package app

import (
	"context"
	"encoding/xml"
	"time"
)

// virtualIOSAdvice 为JSON和XML推送提供同一退款问询答复，最终退款结果仍由Apple决定。
type virtualIOSAdvice struct {
	XMLName    xml.Name `json:"-" xml:"xml"`
	ResultCode int      `json:"result_code" xml:"result_code"`
	ResultInfo string   `json:"result_info" xml:"result_info"`
	Evidence   string   `json:"evidence" xml:"evidence"`
}

// virtualIOSRefundAdvice 在两秒内只读核对原现金充值及已完成服务，不等待外部API或把付款等同履约。
// payOrderID 为已验签官方问询提供的原单号；缺失、冲突、处理中或存储故障均不能成为拒绝退款证据。
func (a *App) virtualIOSRefundAdvice(ctx context.Context, payOrderID string) virtualIOSAdvice {
	advice := virtualIOSAdvice{ResultCode: 0, ResultInfo: "建议退款，最终由Apple审核", Evidence: "未能确认此笔充值对应已完成且无进行中售后的服务；现金退款最终成功后撤回未消费金币并核对相关资金来源"}
	if payOrderID == "" || len(payOrderID) > 128 {
		return advice
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	items, err := list[WalletTopup](ctx, a.DB, "wallet_topups", `WHERE data->>'channel'='virtual' AND (data->>'order_no'=$1 OR data->>'transaction_id'=$1 OR id IN (SELECT id FROM virtual_transactions WHERE data#>>'{order,wx_order_id}'=$1 OR data#>>'{order,wxpay_order_id}'=$1)) LIMIT 2`, payOrderID)
	if err != nil || len(items) != 1 {
		return advice
	}
	p := items[0]
	if p.Status != "paid" || p.RefundedCents != 0 {
		return advice
	}
	proof, err := get[virtualTransaction](ctx, a.DB, "virtual_transactions", p.ID, false)
	if err != nil || proof.Order.OrderType != 7 || proof.ReconciliationRequired || proof.CashRefundCents > 0 || validateVirtualCashOrder(p, proof.Order) != nil {
		return advice
	}
	var fulfilled bool
	err = a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fund_lots l JOIN orders o ON o.id=l.data->>'order_id'
WHERE l.data->>'receipt_id'=$1 AND (l.data->>'amount_cents')::bigint>0
AND l.data->>'movement_id'<>'' AND o.data->>'status'='completed' AND o.data->>'payment_status'='paid'
AND COALESCE(o.data->>'refund_status','') NOT IN ('requested','escalated','processing')
AND COALESCE(o.data->>'completed_at','')<>'')`, walletKey("receipt", "topup", p.ID)).Scan(&fulfilled)
	if err == nil && fulfilled {
		advice.ResultCode = 1
		advice.ResultInfo = "已有实际服务履约证据，请结合消费范围审核"
		advice.Evidence = "此笔已核验现金充值的部分金币来源对应买家已验收完成的服务订单，当前没有进行中的售后；建议按实际已履约消费范围核查退款，最终由Apple决定"
	}
	return advice
}
