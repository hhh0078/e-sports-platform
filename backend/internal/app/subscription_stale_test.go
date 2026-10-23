// Package app 验证订阅提醒在实际业务取消、撤回和审批后失效，过时待办不消耗用户的一次性授权。
package app

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestIntegrationSubscriptionStaleState 验证真实下单取消与退款撤回流程不会发送已过时提醒或消费一次性候选。
func TestIntegrationSubscriptionStaleState(t *testing.T) {
	api := newTestAPI(t)
	for _, item := range []struct {
		name     string
		withdraw bool
	}{{"买家接单前取消", false}, {"退款申请撤回", true}} {
		t.Run(item.name, func(t *testing.T) {
			subscriptionTestReset(t, api)
			settings := subscriptionTestSettings()
			settings.Scenes[1] = SubscriptionScene{Scene: "provider_refund", Enabled: true, TemplateID: settings.Scenes[0].TemplateID, Fields: settings.Scenes[0].Fields}
			if err := putRaw(t.Context(), api.App.DB, "settings", "wechat_subscriptions", settings); err != nil {
				t.Fatal(err)
			}
			subscriptionTestAuthorize(api, "stale-real-flow-123", "accept")
			o := api.newOrder()
			if item.withdraw {
				api.action("worker", o.ID, "accept", "", 200)
				api.action("player", o.ID, "request_refund", "买家申请退款", 200)
				api.action("player", o.ID, "withdraw_refund", "撤回退款申请", 200)
			} else {
				api.action("player", o.ID, "cancel", "取消待接单订单", 200)
			}
			var calls atomic.Int32
			api.App.HTTPClient = subscriptionTestTransport(&calls, func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("过时待办不应外发") })
			for i := 0; i < 10; i++ {
				processed, err := api.App.processSubscriptionOne(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if !processed {
					break
				}
			}
			if calls.Load() != 0 {
				t.Fatalf("过时待办发送%d次", calls.Load())
			}
			var consumed int
			if err := api.App.DB.QueryRow(t.Context(), `SELECT count(*) FROM subscription_authorizations WHERE COALESCE(data->>'consumed_by','')<>''`).Scan(&consumed); err != nil {
				t.Fatal(err)
			}
			if consumed != 0 {
				t.Fatalf("过时提醒消耗%d候选", consumed)
			}
			items, err := list[subscriptionStoredDelivery](t.Context(), api.App.DB, "subscription_deliveries", `WHERE data->>'scene' IN ('provider_order','provider_refund')`)
			if err != nil || len(items) == 0 {
				t.Fatalf("应存在已失效提醒，count=%d err=%v", len(items), err)
			}
			for _, d := range items {
				if d.Status != "expired" {
					t.Fatalf("过时提醒状态%s", d.Status)
				}
			}
		})
	}
}

// TestIntegrationSubscriptionPendingObjects 验证成员与平台审核提醒必须绑定原对象，不能因其它待审对象存在而继续发送。
func TestIntegrationSubscriptionPendingObjects(t *testing.T) {
	api := newTestAPI(t)
	club, err := get[Club](t.Context(), api.App.DB, "clubs", "club-1", false)
	if err != nil {
		clubs, e := list[Club](t.Context(), api.App.DB, "clubs", `WHERE data->>'owner_id'='clubowner' LIMIT 1`)
		if e != nil || len(clubs) == 0 {
			t.Fatalf("测试俱乐部不存在:%v", e)
		}
		club = clubs[0]
	}
	member := ClubMember{ClubID: club.ID, UserID: "outsider", Status: "pending"}
	if err = putRaw(t.Context(), api.App.DB, "members", club.ID+":outsider", member); err != nil {
		t.Fatal(err)
	}
	d := subscriptionStoredDelivery{SubscriptionDelivery: SubscriptionDelivery{Scene: "club_member", UserID: "clubowner"}, ActionPath: "/pages/community/clubs/console?id=" + club.ID + "&member_id=outsider"}
	if ok, e := subscriptionBusinessPending(t.Context(), api.App.DB, d); e != nil || !ok {
		t.Fatalf("有效申请被屏蔽:%v %v", ok, e)
	}
	member.Status = "approved"
	if err = putRaw(t.Context(), api.App.DB, "members", club.ID+":outsider", member); err != nil {
		t.Fatal(err)
	}
	if ok, e := subscriptionBusinessPending(t.Context(), api.App.DB, d); e != nil || ok {
		t.Fatalf("已审批申请仍可发:%v %v", ok, e)
	}
	d.Scene = "admin_club"
	d.UserID = "admin"
	d.ActionPath = "/pages/manage/clubs?id=" + club.ID
	club.Status = "active"
	if err = putRaw(t.Context(), api.App.DB, "clubs", club.ID, club); err != nil {
		t.Fatal(err)
	}
	if ok, e := subscriptionBusinessPending(t.Context(), api.App.DB, d); e != nil || ok {
		t.Fatalf("已审核目标仍可发:%v %v", ok, e)
	}
	d.ActionPath = "/pages/manage/clubs"
	if ok, e := subscriptionBusinessPending(t.Context(), api.App.DB, d); e != nil || ok {
		t.Fatalf("缺失对象不能全表猜测:%v %v", ok, e)
	}
}

// TestIntegrationSubscriptionStaleDuringToken 验证获取官方凭证期间待办变化时，发送前再次复核并归还未使用候选。
func TestIntegrationSubscriptionStaleDuringToken(t *testing.T) {
	api := newTestAPI(t)
	subscriptionTestReset(t, api)
	subscriptionTestAuthorize(api, "token-state-change-123", "accept")
	if err := queueDomainNotice(t.Context(), api.App.DB, subscriptionTestNotice("token-state-change")); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	api.App.HTTPClient = &http.Client{Transport: paymentTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/cgi-bin/stable_token" {
			if _, err := api.App.DB.Exec(t.Context(), `UPDATE orders SET data=data || '{"status":"refunded","payment_status":"refunded","refund_status":"approved"}'::jsonb WHERE id='test-order'`); err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"mock-token-only","expires_in":7200}`))}, nil
		}
		calls.Add(1)
		return nil, fmt.Errorf("状态变化后不得发送")
	})}
	subscriptionTestDrain(t, api)
	d := subscriptionTestDelivery(t, api)
	if d.Status != "expired" || calls.Load() != 0 {
		t.Fatalf("%s calls%d", d.Status, calls.Load())
	}
	var consumed int
	if err := api.App.DB.QueryRow(t.Context(), `SELECT count(*) FROM subscription_authorizations WHERE COALESCE(data->>'consumed_by','')<>''`).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if consumed != 0 {
		t.Fatalf("未发送却消耗%d候选", consumed)
	}
}
