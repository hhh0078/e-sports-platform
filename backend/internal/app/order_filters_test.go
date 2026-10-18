// Package app 验证订单分组先于分页、身份隔离，以及工作台统计与真实列表的一致性。
package app

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// TestIntegrationOrderFiltersBeforePagination 验证首百条之外的进行中及售后订单仍能完整稳定分页。
func TestIntegrationOrderFiltersBeforePagination(t *testing.T) {
	api := newTestAPI(t)
	for _, group := range []string{"working", "refund", "z-completed"} {
		for i := 0; i < 130; i++ {
			o := Order{ID: fmt.Sprintf("%s-%03d", group, i), BuyerID: "player", ProviderID: "worker", Status: "completed", RefundStatus: "none", CreatedAt: "2030-01-01T00:00:00Z"}
			o.OrderNo = o.ID
			// 相同时间使用两种时区写法，必须按实际时间和 ID 排序。
			if i%2 == 0 {
				o.CreatedAt = "2030-01-01T08:00:00+08:00"
			}
			switch group {
			case "working":
				o.Status = []string{"open", "accepted", "in_progress"}[i%3]
			case "refund":
				o.RefundStatus = []string{"requested", "escalated", "processing", "rejected", "approved", ""}[i%6]
				if i%6 >= 4 {
					o.Status = "refunded"
				}
			case "z-completed":
				if i%2 == 0 {
					o.RefundStatus = ""
				}
			}
			if err := put(t.Context(), api.App.DB, "orders", o.ID, o); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 这条同时匹配两个分组，但与当前身份无关，不能穿过角色条件。
	private := Order{ID: "zz-private", OrderNo: "zz-private", BuyerID: "outsider", ProviderID: "clubowner", Status: "in_progress", RefundStatus: "requested", Requirements: "PRIVATE-ORDER-MARKER", CreatedAt: "2031-01-01T00:00:00Z"}
	if err := put(t.Context(), api.App.DB, "orders", private.ID, private); err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"working", "refund"} {
		seen := map[string]bool{}
		for offset := 0; offset <= 144; offset += 24 {
			var page []Order
			api.call("worker", "GET", fmt.Sprintf("/orders?role=provider&status=%s&limit=24&offset=%d", group, offset), nil, 200, &page)
			want := min(24, max(0, 130-offset))
			if len(page) != want {
				t.Fatalf("%s offset=%d 期望%d条，实际%d条", group, offset, want, len(page))
			}
			for i, o := range page {
				wantID := fmt.Sprintf("%s-%03d", group, 129-offset-i)
				if o.ID != wantID || seen[o.ID] {
					t.Fatalf("%s 跨页丢失、重复或身份泄漏：期望%s，实际%s", group, wantID, o.ID)
				}
				seen[o.ID] = true
			}
		}
		if len(seen) != 130 {
			t.Fatalf("%s 分组应完整返回130条，实际%d条", group, len(seen))
		}
	}
}

// TestIntegrationOrderFiltersValidateAndKeepScope 验证所有精确状态、非法参数及个人订单不能因管理员或俱乐部身份扩大范围。
func TestIntegrationOrderFiltersValidateAndKeepScope(t *testing.T) {
	api := newTestAPI(t)
	statuses := []string{"pending_payment", "open", "accepted", "in_progress", "delivered", "completed", "cancelled", "refunded"}
	for _, role := range []string{"buyer", "provider", "private"} {
		for _, status := range statuses {
			o := Order{ID: role + "-" + status, BuyerID: "player", ProviderID: "worker", ClubID: "shanhai", Status: status, RefundStatus: "none", CreatedAt: "2030-01-01T00:00:00Z"}
			o.OrderNo = o.ID
			if role == "buyer" {
				o.BuyerID, o.ProviderID = "worker", "player"
			} else if role == "private" {
				o.BuyerID, o.ProviderID, o.Requirements = "outsider", "clubowner", "PRIVATE-ORDER-MARKER"
			}
			if err := put(t.Context(), api.App.DB, "orders", o.ID, o); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, role := range []string{"buyer", "provider"} {
		for _, status := range statuses {
			var page []Order
			api.call("worker", "GET", "/orders?role="+role+"&status="+status, nil, 200, &page)
			if len(page) != 1 || page[0].ID != role+"-"+status {
				t.Fatalf("精确状态或角色筛选错误：%s/%s %+v", role, status, page)
			}
		}
	}
	for _, path := range []string{"/orders", "/orders?role=all", "/orders?status=all"} {
		var page []Order
		api.call("worker", "GET", path, nil, 200, &page)
		if len(page) != 16 {
			t.Fatalf("双重身份应返回16条本人记录，实际%d条", len(page))
		}
		for _, o := range page {
			if o.BuyerID != "worker" && o.ProviderID != "worker" {
				t.Fatal("个人订单列表泄漏无关用户记录")
			}
		}
	}
	for _, actor := range []string{"admin", "clubowner"} {
		var page []Order
		api.call(actor, "GET", "/orders?role=all", nil, 200, &page)
		want := 0
		if actor == "clubowner" {
			want = 8
		}
		if len(page) != want {
			t.Fatal("平台管理员或俱乐部所有者的个人订单列表不应扩大为全部可查看订单")
		}
		for _, o := range page {
			if o.BuyerID != actor && o.ProviderID != actor {
				t.Fatal("俱乐部所属订单不能混入组织者个人买卖记录")
			}
		}
	}
	for _, pair := range []struct{ key, value string }{{"role", "admin"}, {"role", "seller"}, {"status", "unknown"}, {"status", "requested"}, {"status", "completed' OR 1=1 --"}} {
		api.call("worker", "GET", "/orders?"+pair.key+"="+url.QueryEscape(pair.value), nil, 400, nil)
	}
	api.call("", "GET", "/orders?role=all&status=all", nil, 401, nil)
	code, raw, err := api.request("worker", "GET", "/orders?status=refund", nil)
	if err != nil || code != 200 || strings.Contains(string(raw), "PRIVATE-ORDER-MARKER") {
		t.Fatalf("售后分组泄漏其他用户订单：%s %v", raw, err)
	}
}

// TestIntegrationOrderStatsSeparateRoles 验证买卖统计分离、售后收入扣除及服务总数不受首百条限制。
func TestIntegrationOrderStatsSeparateRoles(t *testing.T) {
	api := newTestAPI(t)
	var initial map[string]int64
	api.call("worker", "GET", "/me/stats", nil, 200, &initial)
	fixtures := []struct {
		buyer, provider, status, refund string
		amount                          int64
	}{
		{"player", "worker", "pending_payment", "none", 100},
		{"player", "worker", "open", "none", 100},
		{"player", "worker", "accepted", "none", 100},
		{"player", "worker", "in_progress", "requested", 100},
		{"player", "worker", "delivered", "none", 100},
		{"player", "worker", "completed", "none", 1100},
		{"player", "worker", "completed", "rejected", 2200},
		{"player", "worker", "completed", "requested", 3300},
		{"player", "worker", "completed", "escalated", 4400},
		{"player", "worker", "completed", "processing", 5500},
		{"player", "worker", "cancelled", "none", 100},
		{"player", "worker", "refunded", "approved", 6600},
		{"worker", "player", "pending_payment", "none", 100},
		{"worker", "player", "accepted", "none", 100},
		{"worker", "player", "completed", "none", 7700},
		{"worker", "player", "completed", "requested", 8800},
		{"worker", "player", "refunded", "approved", 9900},
		{"outsider", "clubowner", "completed", "none", 999999},
	}
	for i, fixture := range fixtures {
		o := Order{ID: fmt.Sprintf("stats-%02d", i), BuyerID: fixture.buyer, ProviderID: fixture.provider, Status: fixture.status, RefundStatus: fixture.refund, AmountCents: fixture.amount, CreatedAt: "2030-01-01T00:00:00Z"}
		o.OrderNo = o.ID
		if err := put(t.Context(), api.App.DB, "orders", o.ID, o); err != nil {
			t.Fatal(err)
		}
	}
	base, err := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 130; i++ {
		s := base
		s.ID, s.Status = fmt.Sprintf("stats-service-%03d", i), []string{"active", "pending", "paused"}[i%3]
		if err = put(t.Context(), api.App.DB, "services", s.ID, s); err != nil {
			t.Fatal(err)
		}
	}
	var stats map[string]int64
	api.call("worker", "GET", "/me/stats", nil, 200, &stats)
	wants := map[string]int64{"buying_count": 2, "working_count": 5, "completed_count": 3, "earnings_cents": 3300, "provider_working_count": 3, "provider_completed_count": 5, "services_count": initial["services_count"] + 130, "buyer_order_count": 5, "completed_order_count": 7}
	for key, want := range wants {
		if stats[key] != want {
			t.Errorf("%s 期望%d，实际%d", key, want, stats[key])
		}
	}
	for _, status := range []string{"working", "completed"} {
		var page []Order
		api.call("worker", "GET", "/orders?role=provider&status="+status, nil, 200, &page)
		if int64(len(page)) != stats["provider_"+status+"_count"] {
			t.Fatalf("%s 工作台数字必须等于点击后的筛选记录数", status)
		}
	}
	for query, key := range map[string]string{"role=buyer&status=all": "buyer_order_count", "role=all&status=completed": "completed_order_count"} {
		var page []Order
		api.call("worker", "GET", "/orders?"+query, nil, 200, &page)
		if int64(len(page)) != stats[key] {
			t.Fatalf("%s 个人页数字必须等于点击后的筛选记录数", key)
		}
	}
	var services []Service
	api.call("worker", "GET", "/services?mine=1", nil, 200, &services)
	if len(services) != 100 || stats["services_count"] <= 130 {
		t.Fatal("服务统计应包含全部状态且不受列表首百条限制")
	}
}
