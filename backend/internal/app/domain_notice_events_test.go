// Package app 验证六类订阅的真实业务入口、接收身份、资金事实、幂等及事务原子性，全部使用隔离测试schema。
package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// domainTestNotices 读取指定精确事件的候选，不把站内消息或真实微信送达混为一谈。
func domainTestNotices(t *testing.T, api *testAPI, event string) []SubscriptionDelivery {
	t.Helper()
	items, err := list[SubscriptionDelivery](t.Context(), api.App.DB, "subscription_deliveries", "WHERE data->>'event'=$1", event)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// assertDomainRecipient 检查单事件只给指定用户生成一条候选，普通用户或其他角色不能收到副本。
func assertDomainRecipient(t *testing.T, api *testAPI, event, scene, user string, count int) {
	t.Helper()
	items := domainTestNotices(t, api, event)
	if len(items) != count {
		t.Fatalf("%s候选数%d，期望%d", event, len(items), count)
	}
	for _, item := range items {
		if item.Scene != scene || item.UserID != user {
			t.Fatalf("事件接收人或场景错误：%+v", item)
		}
	}
}

// TestSubscriptionDomainCatalog 六类事件不能混用模板，读取目录不能修改真实白名单。
func TestSubscriptionDomainCatalog(t *testing.T) {
	if len(subscriptionEventCatalog) != 6 {
		t.Fatal("订阅范围不是用户确认的六类")
	}
	seen := map[string]bool{}
	for scene, events := range subscriptionEventCatalog {
		if len(events) != 1 || !validSubscriptionEvent(scene, events[0].Code) || seen[events[0].Code] {
			t.Fatal("场景事件不唯一")
		}
		seen[events[0].Code] = true
		copy := subscriptionEventsForScene(scene)
		copy[0].Code = "mutated"
		if validSubscriptionEvent(scene, "mutated") {
			t.Fatal("目录泄漏可变引用")
		}
	}
	if validSubscriptionEvent("provider_refund", "order_paid") || validSubscriptionEvent("wallet", "withdrawal_succeeded") {
		t.Fatal("跨场景或未确认事件被开放")
	}
}

// TestIntegrationSubscriptionDomainOrders 用真实交易流程检查付款、接单、退款与组织争议的精确接收人及幂等。
func TestIntegrationSubscriptionDomainOrders(t *testing.T) {
	api := newTestAPI(t)
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1, "requirements": "私密游戏需求不得写入微信摘要"}, 200, &order)
	assertDomainRecipient(t, api, "order_paid", "provider_order", "worker", 0)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{}, 200, nil)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{}, 200, nil)
	assertDomainRecipient(t, api, "order_paid", "provider_order", "worker", 1)
	api.action("worker", order.ID, "accept", "", 200)
	assertDomainRecipient(t, api, "order_accepted", "buyer_accepted", "player", 1)
	api.action("player", order.ID, "request_refund", "需要取消本次服务", 200)
	assertDomainRecipient(t, api, "refund_requested", "provider_refund", "worker", 1)
	api.action("worker", order.ID, "reject_refund", "已经安排服务时间", 200)
	order = api.action("player", order.ID, "escalate", "请平台查看履约证据", 200)
	assertDomainRecipient(t, api, "club_dispute", "club_dispute", "clubowner", 1)
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "club_dispute", "club_dispute", "clubowner", 1)
	api.call("player", "POST", "/orders/"+order.ID+"/messages", map[string]string{"content": "普通聊天不应该触发任何订阅"}, 200, nil)
	all, err := list[map[string]any](t.Context(), api.App.DB, "subscription_deliveries", "")
	if err != nil || len(all) != 4 {
		t.Fatalf("出现未确认事件或普通聊天推送：%d %v", len(all), err)
	}
	for _, item := range all {
		raw, _ := json.Marshal(item)
		if strings.Contains(string(raw), "私密游戏需求") || strings.Contains(string(raw), "请平台查看履约证据") {
			t.Fatal("微信候选泄漏私密原文")
		}
	}
	api.call("worker", "GET", "/messages", nil, 200, nil)
	notice := domainTestNotices(t, api, "order_paid")[0]
	inboxID := "event_" + subscriptionHash("order:"+order.ID+":paid", "provider_order", "worker")
	api.call("worker", "DELETE", "/messages/"+inboxID, nil, 200, nil)
	if err = put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "order_paid", "provider_order", notice.UserID, 1)
}

// TestIntegrationSubscriptionDomainNoFakePayment 历史插入、状态伪实付及演示数据不产生新购买通知。
func TestIntegrationSubscriptionDomainNoFakePayment(t *testing.T) {
	api := newTestAPI(t)
	order := Order{ID: newID(), OrderNo: "NO-CASH-" + newID(), BuyerID: "player", ProviderID: "worker", ServiceID: "delta-service-1", Status: "pending_payment", PaymentStatus: "unpaid", AmountCents: 100, CreatedAt: now(), UpdatedAt: now()}
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	order.PaymentStatus, order.WalletAccounted, order.PaidAt, order.Status = "paid", true, now(), "open"
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "order_paid", "provider_order", "worker", 0)
	legacy := order
	legacy.ID, legacy.OrderNo = newID(), "HISTORY-"+newID()
	if err := put(t.Context(), api.App.DB, "orders", legacy.ID, legacy); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "order_paid", "provider_order", "worker", 0)
	demo := order
	demo.ID, demo.ServiceID, demo.OrderNo = "demo_"+newID(), "demo_service", "DEMO-"+newID()
	if err := put(t.Context(), api.App.DB, "orders", demo.ID, demo); err != nil {
		t.Fatal(err)
	}
	demo.Status = "accepted"
	if err := put(t.Context(), api.App.DB, "orders", demo.ID, demo); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "order_accepted", "buyer_accepted", "player", 0)
}

// TestIntegrationSubscriptionDomainPublicClaim 公开需求支付不广播所有打手，成功被领取后只提醒买家。
func TestIntegrationSubscriptionDomainPublicClaim(t *testing.T) {
	api := newTestAPI(t)
	var order Order
	api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "组队任务需求", "requirements": "希望认真沟通一起完成任务", "amount_cents": 1000}, 200, &order)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{}, 200, nil)
	assertDomainRecipient(t, api, "order_paid", "provider_order", "worker", 0)
	api.action("worker", order.ID, "claim", "", 200)
	assertDomainRecipient(t, api, "order_accepted", "buyer_accepted", "player", 1)
	api.action("player", order.ID, "dispute", "需要平台检查双方约定", 200)
	assertDomainRecipient(t, api, "club_dispute", "club_dispute", "clubowner", 0)
}

// TestIntegrationSubscriptionDomainMemberApply 申请、重复申请与成员审核分别验证，仅首次待办发给组织负责人。
func TestIntegrationSubscriptionDomainMemberApply(t *testing.T) {
	api := newTestAPI(t)
	body := map[string]any{"requirements_accepted": true, "application_note": "申请加入一起认真服务"}
	api.call("outsider", "POST", "/clubs/shanhai/join", body, 200, nil)
	api.call("outsider", "POST", "/clubs/shanhai/join", body, 200, nil)
	assertDomainRecipient(t, api, "member_applied", "club_member", "clubowner", 1)
	api.call("clubowner", "POST", "/clubs/shanhai/members/outsider", map[string]any{"status": "active"}, 200, nil)
	assertDomainRecipient(t, api, "member_applied", "club_member", "clubowner", 1)
	all, err := list[SubscriptionDelivery](t.Context(), api.App.DB, "subscription_deliveries", "")
	if err != nil || len(all) != 1 {
		t.Fatal("成员批准或其它角色被额外推送")
	}
}

// TestIntegrationSubscriptionDomainClubDepositGate 未交创建押金不发平台待办，核验足额并提交组织审核后只提醒管理员。
func TestIntegrationSubscriptionDomainClubDepositGate(t *testing.T) {
	api := newTestAPI(t)
	api.call("admin", "PATCH", "/admin/financial-settings", FinancialSettings{ClubCreationDepositCents: 100}, 200, nil)
	var club Club
	api.call("outsider", "POST", "/clubs", map[string]any{"name": "订阅测试组织", "description": "认真沟通与协作的游戏服务组织", "game_ids": []string{"delta"}}, 200, &club)
	if club.Status != "pending_deposit" {
		t.Fatal("未进入待押金状态")
	}
	assertDomainRecipient(t, api, "admin_club", "admin_club", "admin", 0)
	body := map[string]any{"purpose": "club_platform", "club_id": club.ID, "amount_cents": 100, "request_id": "subscription-club-deposit"}
	api.call("outsider", "POST", "/wallet/topups", body, 200, nil)
	api.call("outsider", "POST", "/wallet/topups", body, 200, nil)
	assertDomainRecipient(t, api, "admin_club", "admin_club", "admin", 1)
	api.call("admin", "PATCH", "/admin/clubs/"+club.ID, map[string]any{"status": "active", "verified": true}, 200, nil)
	assertDomainRecipient(t, api, "admin_club", "admin_club", "admin", 1)
}

// TestIntegrationSubscriptionDomainTransactional 保存和通知共同提交，通知存储失败不能留下业务已修改的半成功结果。
func TestIntegrationSubscriptionDomainTransactional(t *testing.T) {
	api := newTestAPI(t)
	club := Club{ID: newID(), OwnerID: "outsider", Name: "事务测试组织", Status: "pending", CreatedAt: now()}
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err = put(t.Context(), tx, "clubs", club.ID, club); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "admin_club", "admin_club", "admin", 0)
	if _, err = get[Club](t.Context(), api.App.DB, "clubs", club.ID, false); !missing(err) {
		t.Fatal("回滚仍留下组织")
	}
	_, err = api.App.DB.Exec(t.Context(), `CREATE FUNCTION fail_domain_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '测试通知不可用'; END $$; CREATE TRIGGER fail_domain_notice BEFORE INSERT ON subscription_receipts FOR EACH ROW EXECUTE FUNCTION fail_domain_notice()`)
	if err != nil {
		t.Fatal(err)
	}
	if err = put(t.Context(), api.App.DB, "clubs", club.ID, club); err == nil {
		t.Fatal("通知失败未返回错误")
	}
	if _, err = get[Club](t.Context(), api.App.DB, "clubs", club.ID, false); !missing(err) {
		t.Fatal("连接池直写未随通知失败回滚")
	}
}

// TestIntegrationSubscriptionDomainConcurrentPayment 并发实付请求只产生一份新订单通知，订单资金事实不被通知模块改变。
func TestIntegrationSubscriptionDomainConcurrentPayment(t *testing.T) {
	api := newTestAPI(t)
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &order)
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			status, _, err := api.request("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{})
			if err != nil || status != 200 {
				t.Errorf("并发付款请求失败：%d %v", status, err)
			}
		}()
	}
	group.Wait()
	assertDomainRecipient(t, api, "order_paid", "provider_order", "worker", 1)
	var escrows int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM wallet_escrows WHERE id=$1", order.ID).Scan(&escrows); err != nil || escrows != 1 {
		t.Fatal("通知模块破坏实付幂等")
	}
}

// TestIntegrationSubscriptionDomainRefundRounds 主动撤回后再次申请是新待办，原申请回写不会重复消耗订阅。
func TestIntegrationSubscriptionDomainRefundRounds(t *testing.T) {
	api := newTestAPI(t)
	order := financialOrder(api, "player", "delta-service-1", "dev", "")
	api.action("worker", order.ID, "accept", "", 200)
	api.action("player", order.ID, "request_refund", "申请退还本次费用", 200)
	assertDomainRecipient(t, api, "refund_requested", "provider_refund", "worker", 1)
	api.action("player", order.ID, "withdraw_refund", "", 200)
	order = api.action("player", order.ID, "request_refund", "再次申请退还费用", 200)
	assertDomainRecipient(t, api, "refund_requested", "provider_refund", "worker", 2)
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	assertDomainRecipient(t, api, "refund_requested", "provider_refund", "worker", 2)
}

// TestIntegrationSubscriptionDomainConcurrentInsert 同一新对象并发直写只按首次创建通知，缺行锁不会让多份旧值均误判为新申请。
func TestIntegrationSubscriptionDomainConcurrentInsert(t *testing.T) {
	api := newTestAPI(t)
	id := newID()
	var group sync.WaitGroup
	for index := range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			club := Club{ID: id, OwnerID: "outsider", Name: "并发组织申请", Status: "pending", CreatedAt: fmt.Sprintf("2026-10-24T10:00:%02dZ", index)}
			if err := put(t.Context(), api.App.DB, "clubs", id, club); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	var notices int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM inbox WHERE data->>'title'='有俱乐部申请待审核'").Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("并发创建重复生成通知%d %v", notices, err)
	}
}
