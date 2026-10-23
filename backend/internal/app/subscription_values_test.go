// Package app 验证六类模板的固定字段来源和审核投影，不向订阅消息附带原始交易证据或身份秘密。
package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSubscriptionTemplateSceneTitles 限制已有官方模板按六类具体用途绑定，缺少适配模板不能借用其它场景。
func TestSubscriptionTemplateSceneTitles(t *testing.T) {
	for scene, title := range map[string]string{"provider_order": "交易提醒", "provider_refund": "退款申请通知 ", "buyer_accepted": "接单成功提醒", "club_member": "新成员邀请通知", "club_dispute": "订单异常通知", "admin_club": "商家入驻申请通知"} {
		if !subscriptionTemplateTitleAllowed(scene, title) {
			t.Fatalf("适用标题被拒：%s", scene)
		}
	}
	if subscriptionTemplateTitleAllowed("provider_order", "退款申请通知") || subscriptionTemplateTitleAllowed("admin_club", "交易提醒") || subscriptionTemplateTitleAllowed("club_member", "新成员加入通知") {
		t.Fatal("不对应事件的官方模板被错误接受")
	}
}

// TestIntegrationSubscriptionSafeValues 验证真实名称、订单号、时间和晚于入队的审核拒绝不会泄漏旧快照。
func TestIntegrationSubscriptionSafeValues(t *testing.T) {
	api := newTestAPI(t)
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1, "requirements": "不能进入微信模板的私密需求"}, 200, &order)
	d := subscriptionStoredDelivery{SubscriptionDelivery: SubscriptionDelivery{Scene: "buyer_accepted", CreatedAt: now()}, ActionPath: noticeOrderPath(order.ID), Values: map[string]string{"amount": "20.00"}}
	if err := api.App.populateSubscriptionValues(t.Context(), api.App.DB, &d); err != nil {
		t.Fatal(err)
	}
	actor, err := get[User](t.Context(), api.App.DB, "users", "worker", false)
	if err != nil || d.Values["actor_name"] != actor.Nickname || d.Values["service_name"] == "" || d.Values["order_no"] != order.OrderNo || d.Values["created_time"] == "" || d.Values["status"] != "已接单" {
		t.Fatalf("真实模板来源不完整：%+v %v", d.Values, err)
	}
	target := moderationTarget{ID: moderationTargetID("users", actor.ID, "nickname"), TargetType: "users", TargetID: actor.ID, FieldCode: "nickname", Path: []string{"nickname"}, Status: "rejected", Replacement: rejectedContentText}
	if err = putRaw(t.Context(), api.App.DB, "moderation_targets", target.ID, target); err != nil {
		t.Fatal(err)
	}
	if err = api.App.populateSubscriptionValues(t.Context(), api.App.DB, &d); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(d.Values)
	if d.Values["actor_name"] != "平台玩家" || strings.Contains(string(raw), actor.Nickname) || strings.Contains(string(raw), "私密需求") || d.Values["amount"] != "20.00" {
		t.Fatalf("旧原文泄漏或事件金额被篡改：%s", raw)
	}
	if subscriptionBusinessTime("") != "" || subscriptionBusinessTime("bad") != "" {
		t.Fatal("缺失业务时间被编造")
	}
}
