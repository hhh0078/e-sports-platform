// Package app 回归支付配置预检、俱乐部身份、匿名需求隐私、聊天游标与管理搜索。
package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestIntegrationPaymentConfigDoesNotLockCancellation 确认缺少商户配置不会写入预支付标识或阻塞取消。
func TestIntegrationPaymentConfigDoesNotLockCancellation(t *testing.T) {
	api := newTestAPI(t)
	var o Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &o)
	if err := put(t.Context(), api.App.DB, "identities", "player", map[string]string{"openid": "private-openid"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WECHAT_PAY_MCH_ID", "")
	api.App.Config.DevMode = false
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 503, nil)
	saved, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if err != nil || saved.PayOpenID != "" {
		t.Fatalf("未配置时不应建立预支付记录: %+v %v", saved, err)
	}
	api.action("player", o.ID, "cancel", "", 200)
}

// TestIntegrationClubClaimAndPublicPrivacy 验证挂靠状态、接单组织选择和匿名大厅字段白名单。
func TestIntegrationClubClaimAndPublicPrivacy(t *testing.T) {
	api := newTestAPI(t)
	var clubs []Club
	api.call("clubowner", "GET", "/clubs?mine=1", nil, 200, &clubs)
	if len(clubs) != 1 || clubs[0].MembershipStatus != "owner" {
		t.Fatalf("负责人状态错误:%+v", clubs)
	}
	api.call("worker", "GET", "/clubs?mine=1", nil, 200, &clubs)
	if len(clubs) != 1 || clubs[0].MembershipStatus != "active" {
		t.Fatalf("挂靠状态错误:%+v", clubs)
	}
	api.call("worker", "POST", "/clubs/shanhai/leave", map[string]any{}, 200, nil)
	api.call("worker", "GET", "/clubs?mine=1", nil, 200, &clubs)
	if len(clubs) != 1 || clubs[0].MembershipStatus != "left" {
		t.Fatal("退出历史应保留left状态")
	}
	api.call("worker", "POST", "/clubs/shanhai/join", map[string]any{}, 200, nil)
	api.call("worker", "GET", "/clubs?mine=1", nil, 200, &clubs)
	if clubs[0].MembershipStatus != "pending" {
		t.Fatal("重新申请应为pending")
	}
	var o Order
	api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "公开的任务标题", "requirements": "私密暗号 SECRET-PROOF 联系方式不可公开", "amount_cents": 2500}, 200, &o)
	api.call("player", "POST", "/orders/"+o.ID+"/pay", map[string]any{}, 200, nil)
	code, raw, err := api.request("", "GET", "/jobs", nil)
	if err != nil || code != 200 || strings.Contains(string(raw), "SECRET-PROOF") {
		t.Fatalf("匿名需求信息泄漏:%s %v", raw, err)
	}
	var response struct {
		Data []map[string]any `json:"data"`
	}
	json.Unmarshal(raw, &response)
	allowed := map[string]bool{"id": true, "game_id": true, "game_name": true, "title": true, "quantity": true, "amount_cents": true, "status": true, "payment_status": true, "requirements": true, "created_at": true, "unit": true, "unit_price_cents": true}
	for _, job := range response.Data {
		for key := range job {
			if !allowed[key] {
				t.Errorf("匿名响应出现未授权字段 %s", key)
			}
		}
	}
	api.call("worker", "POST", "/orders/"+o.ID+"/actions", map[string]string{"action": "claim", "club_id": "shanhai"}, 403, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/worker", map[string]string{"status": "rejected"}, 200, nil)
	api.call("worker", "GET", "/clubs?mine=1", nil, 200, &clubs)
	if clubs[0].MembershipStatus != "rejected" {
		t.Fatal("拒绝状态未返回")
	}
	api.call("worker", "POST", "/clubs/shanhai/join", map[string]any{}, 200, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/worker", map[string]string{"status": "active"}, 200, nil)
	var claimed Order
	api.call("worker", "POST", "/orders/"+o.ID+"/actions", map[string]string{"action": "claim", "club_id": "shanhai"}, 200, &claimed)
	if claimed.ClubID != "shanhai" {
		t.Fatal("接单组织没有快照")
	}
	api.call("clubowner", "GET", "/orders/"+o.ID, nil, 200, nil)
	api.call("worker", "POST", "/clubs/shanhai/leave", map[string]any{}, 200, nil)
	api.call("clubowner", "GET", "/orders/"+o.ID, nil, 200, nil)
}

// TestIntegrationMessageCursors 验证相同时间戳的稳定分页和休眠超过一百条后的增量补齐。
func TestIntegrationMessageCursors(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	stamp := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	for i := 0; i < 120; i++ {
		m := Message{ID: fmt.Sprintf("old-%s-%03d", o.ID, i), OrderID: o.ID, SenderID: "player", Kind: "text", CreatedAt: stamp, Content: fmt.Sprintf("历史%d", i)}
		if err = put(t.Context(), tx, "messages", m.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	var latest []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages", nil, 200, &latest)
	if len(latest) != 100 {
		t.Fatalf("默认应为最近100条，实际%d", len(latest))
	}
	first, last := latest[0].ID, latest[len(latest)-1].ID
	for i := 0; i < 150; i++ {
		m := Message{ID: fmt.Sprintf("new-%s-%03d", o.ID, i), OrderID: o.ID, SenderID: "worker", Kind: "text", CreatedAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano), Content: "新消息"}
		if err = put(t.Context(), api.App.DB, "messages", m.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	var older []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages?before_id="+first, nil, 200, &older)
	if len(older) != 22 {
		t.Fatalf("新增消息不应挤掉历史页，剩余应22实际%d", len(older))
	}
	var next []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages?after_id="+last, nil, 200, &next)
	if len(next) != 100 {
		t.Fatalf("增量第一页应100实际%d", len(next))
	}
	var final []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages?after_id="+next[len(next)-1].ID, nil, 200, &final)
	if len(final) != 50 {
		t.Fatalf("增量第二页应50实际%d", len(final))
	}
	api.call("player", "GET", "/orders/"+o.ID+"/messages?before_id="+first+"&after_id="+last, nil, 400, nil)
	other := api.newOrder()
	api.call("player", "GET", "/orders/"+other.ID+"/messages?before_id="+first, nil, 400, nil)
}

// TestIntegrationAdminSearchBeforePagination 确认关键词搜索覆盖一百条之外的记录并正确分页。
func TestIntegrationAdminSearchBeforePagination(t *testing.T) {
	api := newTestAPI(t)
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	for i := 0; i < 130; i++ {
		stamp := time.Date(2020, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		title := fmt.Sprintf("catalog-%03d", i)
		if i == 0 {
			title = "needle-unique"
		}
		id := fmt.Sprintf("search-%03d", i)
		u := User{ID: id, Nickname: title, Role: "user", Status: "active", CreatedAt: stamp}
		s := Service{ID: id, Title: title, ProviderID: "worker", GameID: "delta", Status: "active", CreatedAt: stamp}
		o := Order{ID: id, OrderNo: "SEARCH-NO-" + id, Title: title, BuyerID: "player", ProviderID: "worker", AmountCents: 1900, Status: "delivered", PaymentStatus: "paid", RefundStatus: "none", CreatedAt: stamp}
		if i == 0 {
			o.RefundStatus = "requested"
		}
		a := Audit{ID: id, ActorID: "admin", Action: "search.test", TargetID: id, Note: title, CreatedAt: stamp}
		for table, value := range map[string]any{"users": u, "services": s, "orders": o, "audit": a} {
			if err = put(t.Context(), tx, table, id, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"users", "services", "orders", "audit"} {
		var items []map[string]any
		api.call("admin", "GET", "/admin/"+resource+"?q=needle-unique&limit=1", nil, 200, &items)
		if len(items) != 1 || items[0]["id"] != "search-000" {
			t.Fatalf("%s搜索遗漏100条之后旧记录:%+v", resource, items)
		}
		api.call("admin", "GET", "/admin/"+resource+"?q=catalog&limit=2&offset=120", nil, 200, &items)
		if len(items) != 2 {
			t.Fatalf("%s分页错误:%+v", resource, items)
		}
		api.call("admin", "GET", "/admin/"+resource+"?q="+url.QueryEscape("' OR 1=1 --"), nil, 200, &items)
		if len(items) != 0 {
			t.Fatal("搜索内容不应被作为SQL执行")
		}
	}
	var refunds []Order
	api.call("admin", "GET", "/admin/orders?refund=1&q=needle", nil, 200, &refunds)
	if len(refunds) != 1 || refunds[0].ID != "search-000" {
		t.Fatal("退款筛选与搜索不能同时生效")
	}
	var games []Game
	api.call("admin", "GET", "/admin/games?limit=2&offset=2", nil, 200, &games)
	if len(games) != 2 || games[0].SortOrder != 2 || games[1].SortOrder != 3 {
		t.Fatalf("游戏管理分页未生效:%+v", games)
	}
}
