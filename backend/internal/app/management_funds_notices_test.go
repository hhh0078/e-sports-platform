// Package app 验证副管理员押金仍按原支付来源退款，以及原六类通知发送前的实时组织授权。
package app

import "testing"

// TestIntegrationClubDeputyDepositOriginalRoute 验证副管理员缴纳归组织专户，退还仍受认证成员限制且原支付来源不会改成申请人。
func TestIntegrationClubDeputyDepositOriginalRoute(t *testing.T) {
	api := newTestAPI(t)
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": true}, 200, nil)
	var paid struct {
		Payment WalletTopup `json:"payment"`
	}
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": "shanhai", "amount_cents": 500, "request_id": "deputy-platform-deposit"}, 200, &paid)
	if paid.Payment.UserID != "worker" || paid.Payment.AccountID != clubDepositAccount("shanhai") {
		t.Fatal(paid)
	}
	api.call("worker", "POST", "/wallet/topups", map[string]any{"purpose": "club_platform", "club_id": "wild", "amount_cents": 500, "request_id": "deputy-cross-club-deposit"}, 403, nil)
	api.call("worker", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "club_platform", "club_id": "shanhai", "amount_cents": 500, "request_id": "deputy-refund-members-block"}, 409, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/worker", map[string]string{"status": "removed", "review_note": "解除最后认证后按原来源退还"}, 200, nil)
	var refund WalletDepositRefund
	api.call("clubowner", "POST", "/wallet/deposit-refunds", map[string]any{"scope": "club_platform", "club_id": "shanhai", "amount_cents": 500, "request_id": "owner-refund-original-deputy-pay"}, 200, &refund)
	if refund.Status != "success" || refund.UserID != "clubowner" || len(refund.Parts) != 1 || refund.Parts[0].TopupID != paid.Payment.ID {
		t.Fatal(refund)
	}
	stored, err := get[WalletTopup](t.Context(), api.App.DB, "wallet_topups", paid.Payment.ID, false)
	if err != nil || stored.UserID != "worker" || stored.RefundedCents != 500 {
		t.Fatal(stored, err)
	}
	if testWallet(api, "clubowner").RechargeCents != 0 || testWallet(api, "worker").RechargeCents != 0 {
		t.Fatal("组织押金退款不能转成申请人或付款人的平台消费余额")
	}
}

// TestIntegrationClubDeputyNoticeRecheck 只复用已有入会通知，撤权后旧候选不再向副管理员发送。
func TestIntegrationClubDeputyNoticeRecheck(t *testing.T) {
	api := newTestAPI(t)
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": true}, 200, nil)
	api.call("outsider", "POST", "/clubs/shanhai/join", map[string]any{"application_note": "申请加入团队", "requirements_accepted": true}, 200, nil)
	deliveries, err := list[subscriptionStoredDelivery](t.Context(), api.App.DB, "subscription_deliveries", "WHERE data->>'user_id'='worker' AND data->>'scene'='club_member'")
	if err != nil || len(deliveries) != 1 {
		t.Fatal(deliveries, err)
	}
	allowed, err := subscriptionRecipientAllowed(t.Context(), api.App.DB, deliveries[0])
	if err != nil || !allowed {
		t.Fatal("有效副管理员未获得已有入会通知", err)
	}
	api.call("clubowner", "PATCH", "/clubs/shanhai/deputies/worker", map[string]bool{"enabled": false}, 200, nil)
	allowed, err = subscriptionRecipientAllowed(t.Context(), api.App.DB, deliveries[0])
	if err != nil || allowed {
		t.Fatal("撤权后旧通知仍可发送", err)
	}
	if len(subscriptionCatalog) != 6 {
		t.Fatal("本轮不能扩大原六类微信订阅场景")
	}
}
