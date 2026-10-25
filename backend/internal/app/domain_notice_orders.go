// Package app 从真实订单的付款、退款申请、接单和俱乐部争议状态生成四类角色通知。
package app

import (
	"context"
	"fmt"
)

// noticeOrderChange 不解析系统聊天，未付款和演示订单不提醒，重复回调不重复生成通知。
func noticeOrderChange(ctx context.Context, q querier, before, after Order, existed bool) error {
	if requireLiveOrder(after) != nil || after.ID == "" || !existed {
		return nil
	}
	base, path := "order:"+after.ID+":", noticeOrderPath(after.ID)
	emit := func(key, scene, event, title, body string, users ...string) error {
		n := domainNotice(base+key, scene, event, title, body, path, users...)
		n.Values["order_no"] = after.OrderNo
		n.Values["amount"] = fmt.Sprintf("%d.%02d", after.AmountCents/100, after.AmountCents%100)
		return queueDomainNotice(ctx, q, n)
	}
	if before.PaymentStatus != "paid" && after.PaymentStatus == "paid" && after.WalletAccounted && after.PaidAt != "" && after.ProviderID != "" && after.ServiceID != "" {
		escrow, err := get[WalletEscrow](ctx, q, "wallet_escrows", after.ID, false)
		if err != nil && !missing(err) {
			return err
		}
		if err == nil && escrow.Status == "holding" && escrow.BuyerID == after.BuyerID && escrow.ProviderID == after.ProviderID && escrow.AmountCents == after.AmountCents {
			if err = emit("paid", "provider_order", "order_paid", "收到已付款新订单", "有人购买了你的服务，款项已核验，请及时查看订单并接单。", after.ProviderID); err != nil {
				return err
			}
		}
	}
	if before.Status != "accepted" && after.Status == "accepted" && after.PaymentStatus == "paid" {
		if err := emit("accepted", "buyer_accepted", "order_accepted", "订单已接单", "打手已接受订单，请进入订单沟通服务安排。", after.BuyerID); err != nil {
			return err
		}
	}
	if before.RefundStatus != "requested" && after.RefundStatus == "requested" && after.PaymentStatus == "paid" {
		if err := emit("refund-request:"+after.UpdatedAt, "provider_refund", "refund_requested", "收到退款申请", "买家申请了退款，请进入订单查看原因并回应。", after.ProviderID); err != nil {
			return err
		}
	}
	if before.RefundStatus != "escalated" && after.RefundStatus == "escalated" && after.PaymentStatus == "paid" && after.ClubID != "" {
		managers, err := clubManagementRecipients(ctx, q, after.ClubID)
		if err != nil {
			return err
		}
		return emit("club-dispute:"+after.UpdatedAt, "club_dispute", "club_dispute", "俱乐部订单发生争议", "旗下成员关联订单已申请平台介入，请查看订单与原交易证据并协助处理。", managers...)
	}
	return nil
}
