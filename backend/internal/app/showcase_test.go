// Package app 验证演示数据数量、关系、幂等导入及清理时的真实交易保护。
package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// expectedShowcaseCounts 固定本版演示数据规模，防止漏生成某类展示场景。
func expectedShowcaseCounts() map[string]int {
	return map[string]int{"users": 60, "clubs": 6, "members": 42, "services": 130, "orders": 52, "messages": 260, "reviews": 8, "evidence": 60}
}

// TestIntegrationShowcaseSeedAndClear 验证全量导入、幂等、跨游戏场景、公开只读与安全清理。
func TestIntegrationShowcaseSeedAndClear(t *testing.T) {
	api := newTestAPI(t)
	original, err := get[map[string]any](t.Context(), api.App.DB, "users", "player", false)
	if err != nil {
		t.Fatal(err)
	}
	created, err := api.App.SeedShowcase(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(created, expectedShowcaseCounts()) {
		t.Fatalf("演示数量不完整：%+v", created)
	}
	var firstOrder []byte
	if err = api.App.DB.QueryRow(t.Context(), "SELECT data FROM orders WHERE id='demo_order_001'").Scan(&firstOrder); err != nil {
		t.Fatal(err)
	}
	again, err := api.App.SeedShowcase(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for table, count := range again {
		if count != 0 {
			t.Fatalf("重复导入覆盖了%s：%d", table, count)
		}
	}
	var secondOrder []byte
	api.App.DB.QueryRow(t.Context(), "SELECT data FROM orders WHERE id='demo_order_001'").Scan(&secondOrder)
	if string(firstOrder) != string(secondOrder) {
		t.Fatal("幂等导入改写了原有演示记录")
	}
	counts, err := api.App.ShowcaseStatus(t.Context())
	if err != nil || !reflect.DeepEqual(counts, expectedShowcaseCounts()) {
		t.Fatalf("统计不符：%+v %v", counts, err)
	}
	var gameCount, stateCount, adminCount, identityCount int
	api.App.DB.QueryRow(t.Context(), "SELECT count(DISTINCT data->>'game_id') FROM services WHERE left(id,5)='demo_'").Scan(&gameCount)
	api.App.DB.QueryRow(t.Context(), "SELECT count(DISTINCT concat_ws(':',data->>'status',data->>'refund_status')) FROM orders WHERE left(id,5)='demo_'").Scan(&stateCount)
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM users WHERE left(id,5)='demo_' AND (data->>'role'='admin' OR data->>'phone_verified'='true' OR COALESCE(data->>'phone_masked','')<>'')").Scan(&adminCount)
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM identities WHERE left(id,5)='demo_'").Scan(&identityCount)
	if gameCount != 13 || stateCount < 10 || adminCount != 0 || identityCount != 0 {
		t.Fatalf("演示范围或身份边界错误：games=%d states=%d admins=%d identities=%d", gameCount, stateCount, adminCount, identityCount)
	}
	api.call("", "POST", "/auth/dev", map[string]any{"user_id": "demo_user_001"}, 400, nil)
	api.call("player", "POST", "/orders", map[string]any{"service_id": "demo_service_001", "quantity": 1}, 400, nil)
	for _, operation := range []struct{ method, path string }{
		{"POST", "/orders/demo_order_001/pay"},
		{"POST", "/orders/demo_order_001/sync-payment"},
		{"POST", "/orders/demo_order_002/actions"},
		{"POST", "/orders/demo_order_001/messages"},
		{"POST", "/admin/orders/demo_order_012/resolve"},
		{"POST", "/admin/orders/demo_order_013/retry-refund"},
		{"PATCH", "/admin/users/demo_user_001"},
		{"PATCH", "/clubs/demo_club_001"},
		{"POST", "/services/demo_service_001/pause"},
	} {
		api.call("admin", operation.method, operation.path, map[string]any{}, 400, nil)
	}
	api.call("", "GET", "/players/demo_user_007", nil, 200, nil)
	api.call("", "GET", "/clubs/demo_club_001", nil, 200, nil)
	var messages []Message
	api.call("admin", "GET", "/orders/demo_order_001/messages", nil, 200, &messages)
	roles := map[string]bool{}
	for _, message := range messages {
		roles[message.SenderRole] = true
		if !strings.Contains(message.Content, "演示") {
			t.Fatal("演示聊天缺少明确标记")
		}
	}
	if len(messages) != 5 || !roles["buyer"] || !roles["provider"] || !roles["club_admin"] || !roles["admin"] {
		t.Fatal("四方聊天示例不完整")
	}
	var services []Service
	api.call("", "GET", "/services?game_id=delta", nil, 200, &services)
	shown := 0
	for _, service := range services {
		if strings.HasPrefix(service.ID, "demo_") {
			shown++
			if !strings.Contains(service.Title, "演示") {
				t.Fatal("公开服务缺少演示标识")
			}
		}
	}
	if shown != 10 {
		t.Fatalf("三角洲演示服务应有10条，实际%d", shown)
	}
	removed, err := api.App.ClearShowcase(t.Context())
	if err != nil || !reflect.DeepEqual(removed, expectedShowcaseCounts()) {
		t.Fatalf("清理失败：%+v %v", removed, err)
	}
	after, err := get[map[string]any](t.Context(), api.App.DB, "users", "player", false)
	if err != nil || !reflect.DeepEqual(original, after) {
		t.Fatal("真实用户被演示命令改写")
	}
	counts, err = api.App.ShowcaseStatus(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for table, count := range counts {
		if count != 0 {
			t.Fatalf("清理后%s仍有%d条", table, count)
		}
	}
	if _, err = api.App.ClearShowcase(t.Context()); err != nil {
		t.Fatalf("再次清理应幂等：%v", err)
	}
}

// TestIntegrationShowcaseProtectsRealReferences 验证真实订单引用和真实参与者混入都会阻止整笔清理。
func TestIntegrationShowcaseProtectsRealReferences(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.SeedShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	order := Order{ID: "real-order-with-demo-reference", OrderNo: "REAL-SAFE-001", BuyerID: "player", ProviderID: "worker", ServiceID: "demo_service_001", Status: "completed", PaymentStatus: "paid", RefundStatus: "none"}
	if err := put(t.Context(), api.App.DB, "orders", order.ID, order); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.ClearShowcase(t.Context()); err == nil {
		t.Fatal("真实交易引用时不能删除演示服务")
	}
	counts, _ := api.App.ShowcaseStatus(t.Context())
	if !reflect.DeepEqual(counts, expectedShowcaseCounts()) {
		t.Fatal("清理拒绝后发生部分删除")
	}
	if _, err := get[Order](t.Context(), api.App.DB, "orders", order.ID, false); err != nil {
		t.Fatal("真实订单被删除")
	}
	api.App.DB.Exec(t.Context(), "DELETE FROM orders WHERE id=$1", order.ID)
	if _, err := api.App.DB.Exec(t.Context(), "UPDATE orders SET data=jsonb_set(data,'{buyer_id}','\"player\"') WHERE id='demo_order_001'"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.ClearShowcase(t.Context()); err == nil {
		t.Fatal("演示订单混入真实买家时不能删除")
	}
	api.App.DB.Exec(t.Context(), "UPDATE orders SET data=jsonb_set(data,'{buyer_id}','\"demo_user_043\"') WHERE id='demo_order_001'")
	if _, err := api.App.DB.Exec(t.Context(), "UPDATE users SET data=jsonb_set(data,'{avatar}','\"/api/media/demo_avatar_001\"') WHERE id='player'"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.ClearShowcase(t.Context()); err == nil {
		t.Fatal("真实用户使用演示图片时不能删除图片")
	}
	api.App.DB.Exec(t.Context(), "UPDATE users SET data=jsonb_set(data,'{avatar}','\"\"') WHERE id='player'")
	if _, err := api.App.ClearShowcase(t.Context()); err != nil {
		t.Fatalf("移除真实引用后应允许安全清理：%v", err)
	}
}

// TestIntegrationShowcaseNamespaceCollision 验证同名前缀不等于演示授权，碰撞时不覆盖原数据。
func TestIntegrationShowcaseNamespaceCollision(t *testing.T) {
	api := newTestAPI(t)
	original := map[string]any{"id": "demo_user_001", "nickname": "必须保留的原有记录", "role": "user", "status": "active"}
	if err := put(t.Context(), api.App.DB, "users", "demo_user_001", original); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.SeedShowcase(t.Context()); err == nil {
		t.Fatal("同名真实记录不能被演示导入覆盖")
	}
	stored, err := get[map[string]any](t.Context(), api.App.DB, "users", "demo_user_001", false)
	if err != nil || !reflect.DeepEqual(stored, original) {
		t.Fatal("冲突记录被改写")
	}
	counts, _ := api.App.ShowcaseStatus(t.Context())
	for _, count := range counts {
		if count != 0 {
			t.Fatal("导入失败留下了部分演示数据")
		}
	}
	if _, err = api.App.ClearShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	stored, err = get[map[string]any](t.Context(), api.App.DB, "users", "demo_user_001", false)
	raw, _ := json.Marshal(stored)
	if err != nil || !strings.Contains(string(raw), "必须保留") {
		t.Fatal("仅带demo_前缀的非演示记录被删除")
	}
}

// TestIntegrationShowcaseProtectsEvidenceAssociations 验证演示证据混入真实订单、用户或俱乐部时整笔拒绝清理。
func TestIntegrationShowcaseProtectsEvidenceAssociations(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.SeedShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	liveOrder := Order{ID: "real-evidence-order", OrderNo: "REAL-EVIDENCE-001", BuyerID: "player", ProviderID: "worker", Status: "completed"}
	if err := put(t.Context(), api.App.DB, "orders", liveOrder.ID, liveOrder); err != nil {
		t.Fatal(err)
	}
	if err := put(t.Context(), api.App.DB, "clubs", "real-evidence-club", Club{ID: "real-evidence-club", OwnerID: "clubowner"}); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct{ table, field, value string }{
		{"messages", "order_id", liveOrder.ID}, {"messages", "sender_id", "player"},
		{"reviews", "order_id", liveOrder.ID},
		{"evidence", "order_id", liveOrder.ID}, {"evidence", "owner_id", "player"}, {"evidence", "club_id", "real-evidence-club"},
	} {
		t.Run(scenario.table+"_"+scenario.field, func(t *testing.T) {
			var id string
			var before []byte
			if err := api.App.DB.QueryRow(t.Context(), "SELECT id,data FROM "+validTable(scenario.table)+" WHERE data->>'is_demo'='true' ORDER BY id LIMIT 1").Scan(&id, &before); err != nil {
				t.Fatal(err)
			}
			if _, err := api.App.DB.Exec(t.Context(), "UPDATE "+validTable(scenario.table)+" SET data=jsonb_set(data,ARRAY[$1::text],to_jsonb($2::text)) WHERE id=$3", scenario.field, scenario.value, id); err != nil {
				t.Fatal(err)
			}
			defer api.App.DB.Exec(t.Context(), "UPDATE "+validTable(scenario.table)+" SET data=$1 WHERE id=$2", before, id)
			if _, err := api.App.ClearShowcase(t.Context()); err == nil {
				t.Fatal("关联真实业务的演示证据被清理")
			}
			counts, err := api.App.ShowcaseStatus(t.Context())
			if err != nil || !reflect.DeepEqual(counts, expectedShowcaseCounts()) {
				t.Fatal("清理拒绝后出现部分删除或演示标记丢失")
			}
		})
	}
	if _, err := api.App.ClearShowcase(t.Context()); err != nil {
		t.Fatalf("移除真实证据关联后应允许清理：%v", err)
	}
	if _, err := get[Order](t.Context(), api.App.DB, "orders", liveOrder.ID, false); err != nil {
		t.Fatal("清理影响真实订单")
	}
}

// TestIntegrationOverviewExcludesShowcase 验证演示导入不会虚增经营金额、用户数量及待处理任务。
func TestIntegrationOverviewExcludesShowcase(t *testing.T) {
	api := newTestAPI(t)
	paid := Order{ID: "real-revenue-order", OrderNo: "REAL-REVENUE-001", BuyerID: "player", Status: "completed", PaymentStatus: "paid", RefundStatus: "none", AmountCents: 12345}
	if err := put(t.Context(), api.App.DB, "orders", paid.ID, paid); err != nil {
		t.Fatal(err)
	}
	var before map[string]int64
	api.call("admin", "GET", "/admin/overview", nil, 200, &before)
	if before["revenue_cents"] != paid.AmountCents {
		t.Fatal("真实完成订单未计入经营金额")
	}
	if _, err := api.App.SeedShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"clubs", "services"} {
		if _, err := api.App.DB.Exec(t.Context(), "UPDATE "+validTable(table)+" SET data=jsonb_set(data,'{status}','\"pending\"') WHERE data->>'is_demo'='true'"); err != nil {
			t.Fatal(err)
		}
	}
	var after map[string]int64
	api.call("admin", "GET", "/admin/overview", nil, 200, &after)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("演示数据污染经营统计：before=%v after=%v", before, after)
	}
}
