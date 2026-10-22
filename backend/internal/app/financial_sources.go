// Package app 逐笔记录已核验付款根、资金批次与不可变分配，余额消费、结算及提现始终保留原始来源。
package app

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// FundingReceipt 是由服务端核验支付成功后建立的不可变收款根凭证。
type FundingReceipt struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	MerchantID    string `json:"merchant_id"`
	AppID         string `json:"app_id"`
	PaymentID     string `json:"payment_id"`
	TransactionID string `json:"transaction_id"`
	AmountCents   int64  `json:"amount_cents"`
	Channel       string `json:"channel"`
	CreatedAt     string `json:"created_at"`
}

// FundingLot 按账户和用途持有来源金额；原始金额不可增发，剩余额只在事务中转移。
type FundingLot struct {
	ID             string `json:"id"`
	AccountID      string `json:"account_id"`
	Bucket         string `json:"bucket"`
	ReceiptID      string `json:"receipt_id"`
	ParentID       string `json:"parent_id,omitempty"`
	MovementID     string `json:"movement_id,omitempty"`
	OrderID        string `json:"order_id,omitempty"`
	ReturnBucket   string `json:"return_bucket,omitempty"`
	AmountCents    int64  `json:"amount_cents"`
	RemainingCents int64  `json:"remaining_cents"`
	CreatedAt      string `json:"created_at"`
}

// FundingPart 绑定一次资金转移实际消耗的父批次和整数金额。
type FundingPart struct {
	LotID       string `json:"lot_id"`
	AmountCents int64  `json:"amount_cents"`
}

// FundingMovement 追加保存转移输入输出，已落账后不可覆盖或删除。
type FundingMovement struct {
	ID        string        `json:"id"`
	Inputs    []FundingPart `json:"inputs"`
	Outputs   []FundingPart `json:"outputs"`
	CreatedAt string        `json:"created_at"`
}

// fundDestination 描述一次分账的目标账号、用途及金额。
type fundDestination struct {
	AccountID, Bucket, OrderID string
	AmountCents                int64
	PreserveReturn             bool
}

// addFundingRoot 在支付验证事务中建立唯一收款凭证和原始批次，重复根必须逐项一致。
func addFundingRoot(ctx context.Context, q querier, kind, id, transaction string, amount int64, channel, account, bucket, order string, merchant ...string) error {
	rid := walletKey("receipt", kind, id)
	existing, e := get[FundingReceipt](ctx, q, "fund_receipts", rid, false)
	if e == nil {
		if existing.TransactionID != transaction || existing.AmountCents != amount || existing.Channel != channel {
			return fail(409, "原始付款资金凭证冲突")
		}
		return nil
	}
	if !missing(e) {
		return e
	}
	if amount <= 0 || transaction == "" {
		return fail(409, "付款缺少有效资金凭证")
	}
	r := FundingReceipt{ID: rid, Kind: kind, PaymentID: id, TransactionID: transaction, AmountCents: amount, Channel: channel, CreatedAt: now()}
	if len(merchant) >= 2 {
		r.MerchantID = merchant[0]
		r.AppID = merchant[1]
	}
	if e = put(ctx, q, "fund_receipts", rid, r); e != nil {
		return e
	}
	lot := FundingLot{ID: walletKey("root-lot", rid), AccountID: account, Bucket: bucket, ReceiptID: rid, OrderID: order, AmountCents: amount, RemainingCents: amount, CreatedAt: r.CreatedAt}
	return put(ctx, q, "fund_lots", lot.ID, lot)
}

// fundingLots 在所属账户事务锁下按先入先出读取可用来源，行锁防止并发重复占用。
func fundingLots(ctx context.Context, q querier, account, bucket string) ([]FundingLot, error) {
	return list[FundingLot](ctx, q, "fund_lots", "WHERE data->>'account_id'=$1 AND data->>'bucket'=$2 AND (data->>'remaining_cents')::bigint>0 ORDER BY data->>'created_at',id FOR UPDATE", account, bucket)
}

// moveFunding 将来源批次逐分守恒转至多个目标；目标为空时表示已核验的外部退款或提现支出。
func moveFunding(ctx context.Context, q querier, event, account, bucket string, amount int64, targets []fundDestination) error {
	mid := walletKey("fund-move", event)
	old, e := get[FundingMovement](ctx, q, "fund_movements", mid, false)
	if e == nil {
		var total int64
		for _, p := range old.Inputs {
			total += p.AmountCents
		}
		if total != amount {
			return fail(409, "资金业务号对应金额冲突")
		}
		return nil
	}
	if !missing(e) {
		return e
	}
	if amount == 0 {
		return nil
	}
	var requested int64
	for _, t := range targets {
		if t.AmountCents < 0 {
			return fail(409, "分账金额无效")
		}
		requested += t.AmountCents
	}
	if len(targets) > 0 && requested != amount {
		return fail(409, "资金分账金额不守恒")
	}
	lots, e := fundingLots(ctx, q, account, bucket)
	if e != nil {
		return e
	}
	var capacity int64
	for _, l := range lots {
		capacity += l.RemainingCents
	}
	if capacity < amount {
		return fail(409, "资金来源不足或历史账目缺少支付凭证，请联系平台核账")
	}
	m := FundingMovement{ID: mid, Inputs: []FundingPart{}, Outputs: []FundingPart{}, CreatedAt: now()}
	remaining := amount
	targetIndex := 0
	targetRemaining := int64(0)
	if len(targets) > 0 {
		targetRemaining = targets[0].AmountCents
	}
	for _, l := range lots {
		take := l.RemainingCents
		if take > remaining {
			take = remaining
		}
		if take <= 0 {
			continue
		}
		l.RemainingCents -= take
		if e = put(ctx, q, "fund_lots", l.ID, l); e != nil {
			return e
		}
		m.Inputs = append(m.Inputs, FundingPart{l.ID, take})
		piece := take
		for piece > 0 && len(targets) > 0 {
			for targetIndex < len(targets) && targetRemaining == 0 {
				targetIndex++
				if targetIndex < len(targets) {
					targetRemaining = targets[targetIndex].AmountCents
				}
			}
			if targetIndex >= len(targets) {
				return fail(409, "资金输出分配异常")
			}
			t := targets[targetIndex]
			part := piece
			if part > targetRemaining {
				part = targetRemaining
			}
			out := FundingLot{ID: walletKey("fund-output", mid, fmt.Sprint(len(m.Outputs))), AccountID: t.AccountID, Bucket: t.Bucket, ReceiptID: l.ReceiptID, ParentID: l.ID, MovementID: mid, OrderID: t.OrderID, AmountCents: part, RemainingCents: part, CreatedAt: m.CreatedAt}
			if out.OrderID == "" {
				out.OrderID = l.OrderID
			}
			if t.PreserveReturn {
				out.ReturnBucket = l.Bucket
			}
			if e = put(ctx, q, "fund_lots", out.ID, out); e != nil {
				return e
			}
			m.Outputs = append(m.Outputs, FundingPart{out.ID, part})
			piece -= part
			targetRemaining -= part
		}
		remaining -= take
		if remaining == 0 {
			break
		}
	}
	return put(ctx, q, "fund_movements", mid, m)
}

// restoreEscrowFunding 按原始资金用途恢复余额来源，充值款仍不可提现，服务收入仍可提现。
func restoreEscrowFunding(ctx context.Context, q querier, o Order, escrow WalletEscrow) error {
	lots, e := fundingLots(ctx, q, "escrow:"+o.ID, "escrow")
	if e != nil {
		return e
	}
	var recharge, income int64
	for _, l := range lots {
		switch l.ReturnBucket {
		case "recharge":
			recharge += l.RemainingCents
		case "available":
			income += l.RemainingCents
		default:
			return fail(409, "余额退款资金来源缺少原付款分区")
		}
	}
	if len(lots) == 0 || recharge != escrow.RechargeUsed || income != escrow.IncomeUsed || recharge+income != escrow.AmountCents || o.BalanceRechargeCents != recharge || o.BalanceIncomeCents != income {
		return fail(409, "余额退款来源不足或与原付款分区不一致，请平台核账")
	}
	for _, l := range lots {
		bucket := l.ReturnBucket
		if bucket == "" {
			return fail(409, "余额退款缺少原支付分区")
		}
		if e = moveFundingLot(ctx, q, "refund:"+o.ID+":"+l.ID, l, o.BuyerID, bucket); e != nil {
			return e
		}
	}
	return nil
}

// moveFundingLot 精确转回单一来源批次，避免混合余额退款改变原充值和收入归属。
func moveFundingLot(ctx context.Context, q querier, event string, l FundingLot, account, bucket string) error {
	mid := walletKey("fund-move", event)
	if _, e := get[FundingMovement](ctx, q, "fund_movements", mid, false); e == nil {
		return nil
	}
	amount := l.RemainingCents
	l.RemainingCents = 0
	if e := put(ctx, q, "fund_lots", l.ID, l); e != nil {
		return e
	}
	out := FundingLot{ID: walletKey("fund-output", mid, "0"), AccountID: account, Bucket: bucket, ReceiptID: l.ReceiptID, ParentID: l.ID, MovementID: mid, OrderID: l.OrderID, AmountCents: amount, RemainingCents: amount, CreatedAt: now()}
	if parent, e := get[FundingLot](ctx, q, "fund_lots", l.ParentID, false); e == nil {
		out.OrderID = parent.OrderID
	}
	if e := put(ctx, q, "fund_lots", out.ID, out); e != nil {
		return e
	}
	return put(ctx, q, "fund_movements", mid, FundingMovement{ID: mid, Inputs: []FundingPart{{l.ID, amount}}, Outputs: []FundingPart{{out.ID, amount}}, CreatedAt: now()})
}

// verifyFundingLot 沿不可变父分配验证根付款与金额容量，历史断链或开发款不得混入真实提现。
func verifyFundingLot(ctx context.Context, q querier, l FundingLot, dev bool, seen map[string]bool) error {
	if len(seen) >= 256 {
		return fail(409, "资金来源链路过长，请平台人工核账后处理")
	}
	if seen[l.ID] {
		return fail(409, "资金来源出现循环")
	}
	seen[l.ID] = true
	defer delete(seen, l.ID)
	if l.AmountCents <= 0 || l.RemainingCents < 0 || l.RemainingCents > l.AmountCents {
		return fail(409, "来源批次金额异常")
	}
	var consumed, currentRemaining int64
	if e := q.QueryRow(ctx, `SELECT (l.data->>'remaining_cents')::bigint,(SELECT COALESCE(sum((part->>'amount_cents')::bigint),0) FROM fund_movements CROSS JOIN LATERAL jsonb_array_elements(data->'inputs') part WHERE data->'inputs' @> jsonb_build_array(jsonb_build_object('lot_id',$1::text)) AND part->>'lot_id'=$1) FROM fund_lots l WHERE l.id=$1`, l.ID).Scan(&currentRemaining, &consumed); e != nil {
		return e
	}
	if consumed+currentRemaining != l.AmountCents {
		return fail(409, "资金来源容量与已分配金额不守恒")
	}
	r, e := get[FundingReceipt](ctx, q, "fund_receipts", l.ReceiptID, false)
	if e != nil {
		return fail(409, "原始收款凭证缺失")
	}
	if !dev && r.Channel == "dev" {
		return fail(409, "模拟资金禁止真实提现")
	}
	if l.ParentID != "" {
		parent, e := get[FundingLot](ctx, q, "fund_lots", l.ParentID, false)
		if e != nil || parent.ReceiptID != l.ReceiptID {
			return fail(409, "资金父批次断裂")
		}
		if l.OrderID != "" && l.MovementID == walletKey("fund-move", "settlement:"+l.OrderID) {
			order, e := get[Order](ctx, q, "orders", l.OrderID, false)
			if e != nil || !walletIncomeEligible(order, time.Now()) {
				return fail(409, "服务收入的交付、售后或支付状态无法核对")
			}
		}
		m, e := get[FundingMovement](ctx, q, "fund_movements", l.MovementID, false)
		if e != nil {
			return fail(409, "资金分配凭证缺失")
		}
		found := false
		parentInput := int64(0)
		var in, out int64
		for _, p := range m.Inputs {
			in += p.AmountCents
			if p.LotID == parent.ID {
				parentInput += p.AmountCents
			}
		}
		for _, p := range m.Outputs {
			out += p.AmountCents
			if p.LotID == l.ID && p.AmountCents == l.AmountCents {
				found = true
			}
		}
		parentOutput := int64(0)
		for _, part := range m.Outputs {
			child, e := get[FundingLot](ctx, q, "fund_lots", part.LotID, false)
			if e != nil {
				return fail(409, "分配输出批次缺失")
			}
			if child.ParentID == parent.ID {
				parentOutput += part.AmountCents
			}
		}
		if !found || in != out || parentInput != parentOutput || parentInput < l.AmountCents {
			return fail(409, "资金分配不守恒")
		}
		return verifyFundingLot(ctx, q, parent, dev, seen)
	}
	if l.AmountCents != r.AmountCents {
		return fail(409, "原始资金批次与支付金额不符")
	}
	switch r.Kind {
	case "topup":
		p, e := get[WalletTopup](ctx, q, "wallet_topups", r.PaymentID, false)
		if e != nil || p.TransactionID != r.TransactionID || p.AmountCents != r.AmountCents || p.Purpose != "balance" || p.Status != "paid" || p.MerchantID != r.MerchantID || p.AppID != r.AppID {
			return fail(409, "充值收款凭证未能核对")
		}
	case "order":
		o, e := get[Order](ctx, q, "orders", r.PaymentID, false)
		if e != nil || o.AmountCents != r.AmountCents || o.PaymentStatus != "paid" || o.PaymentChannel != r.Channel || (r.Channel != "dev" && (o.PaymentTransactionID != r.TransactionID || o.PaymentMerchantID != r.MerchantID || o.PaymentAppID != r.AppID)) {
			return fail(409, "订单收款凭证未能核对")
		}
	default:
		return fail(409, "不支持的收款来源")
	}
	return nil
}

// withdrawalSource 审核详情只返回资金溯源所需业务编号，不返回openid或支付密钥。
type withdrawalSource struct {
	SourceID    string `json:"source_id"`
	OrderID     string `json:"order_id"`
	PaymentID   string `json:"payment_id"`
	AmountCents int64  `json:"amount_cents"`
	Channel     string `json:"channel"`
}

// auditWithdrawalSources 对提现冻结的每一批次做来源核验，金额不足时始终拒绝出款。
func auditWithdrawalSources(ctx context.Context, q querier, v WalletWithdrawal, dev bool) ([]withdrawalSource, []string, error) {
	lots, e := list[FundingLot](ctx, q, "fund_lots", "WHERE data->>'account_id'=$1 AND data->>'bucket'='withdrawal' ORDER BY id", "withdrawal:"+v.ID)
	if e != nil {
		return nil, nil, e
	}
	sources := []withdrawalSource{}
	issues := []string{}
	var total int64
	for _, l := range lots {
		amount := l.RemainingCents
		if contains([]string{"success", "failed", "rejected"}, v.Status) {
			amount = l.AmountCents
		}
		if amount == 0 {
			continue
		}
		total += amount
		if e = verifyFundingLot(ctx, q, l, dev, map[string]bool{}); e != nil {
			issues = append(issues, e.Error())
		}
		r, _ := get[FundingReceipt](ctx, q, "fund_receipts", l.ReceiptID, false)
		if !dev && (r.MerchantID == "" || r.AppID == "" || r.MerchantID != v.MerchantID || r.AppID != v.AppID) {
			issues = append(issues, "原收款商户或小程序与本次出款账户不一致")
		}
		sources = append(sources, withdrawalSource{l.ID, l.OrderID, r.PaymentID, amount, r.Channel})
	}
	if total != v.AmountCents {
		issues = append(issues, "提现金额与已占用资金来源总额不一致")
	}
	sort.Strings(issues)
	return sources, issues, nil
}

// recordFundingAudit 追加提现申请或执行时的核验快照，历史审核证据不能被后续查询改写。
func recordFundingAudit(ctx context.Context, q querier, v WalletWithdrawal, phase string, sources []withdrawalSource, issues []string) error {
	id := walletKey("fund-audit", v.ID, phase)
	if _, e := get[map[string]any](ctx, q, "fund_audits", id, false); e == nil {
		return nil
	} else if !missing(e) {
		return e
	}
	return put(ctx, q, "fund_audits", id, map[string]any{"id": id, "withdrawal_id": v.ID, "phase": phase, "sources": sources, "issues": issues, "passed": len(issues) == 0, "amount_cents": v.AmountCents, "created_at": now()})
}
