// Package app 验证服务评价触底分页突破旧百条上限，同时保留公开可见性与退款过滤。
package app

import (
	"fmt"
	"net/url"
	"testing"
)

// TestIntegrationReviewPagination 验证同时间戳的评价稳定分页、不漏不重，售后评价和未公开服务不越权。
func TestIntegrationReviewPagination(t *testing.T) {
	api := newTestAPI(t)
	const serviceID = "review-pagination-service"
	s := Service{ID: serviceID, ProviderID: "worker", GameID: "delta", Status: "active", Title: "分页评价测试"}
	if err := put(t.Context(), api.App.DB, "services", s.ID, s); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 106; i++ {
		id := fmt.Sprintf("review-pagination-%03d", i)
		o := Order{ID: id, OrderNo: id, ServiceID: serviceID, BuyerID: "player", ProviderID: "worker", Status: "completed", RefundStatus: "none"}
		if i == 105 {
			o.RefundStatus = "requested"
		}
		if err := put(t.Context(), api.App.DB, "orders", id, o); err != nil {
			t.Fatal(err)
		}
		review := Review{ID: id, OrderID: id, BuyerName: "测试玩家", Rating: 5, Content: "分页测试评价", CreatedAt: "2026-10-24T00:00:00Z"}
		if err := put(t.Context(), api.App.DB, "reviews", id, review); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for offset := 0; offset < 120; offset += 20 {
		var got Service
		api.call("", "GET", fmt.Sprintf("/services/%s?limit=20&offset=%d", url.PathEscape(serviceID), offset), nil, 200, &got)
		want := 20
		if offset == 100 {
			want = 5
		}
		if len(got.Reviews) != want {
			t.Fatalf("offset=%d期望%d条评价，实际%d", offset, want, len(got.Reviews))
		}
		for _, row := range got.Reviews {
			if seen[row.ID] {
				t.Fatalf("分页重复：%s", row.ID)
			}
			if row.ID == "review-pagination-105" {
				t.Fatal("退款中评价不可公开")
			}
			seen[row.ID] = true
		}
	}
	if len(seen) != 105 {
		t.Fatalf("未完整加载评价：%d", len(seen))
	}
	var legacy, empty Service
	api.call("", "GET", "/services/"+serviceID, nil, 200, &legacy)
	if len(legacy.Reviews) != 100 {
		t.Fatalf("无显式分页应保留百条兼容默认，实际%d", len(legacy.Reviews))
	}
	api.call("", "GET", "/services/"+serviceID+"?limit=20&offset=120", nil, 200, &empty)
	if len(empty.Reviews) != 0 {
		t.Fatal("评价到底后应为空")
	}
	s.Status = "paused"
	if err := put(t.Context(), api.App.DB, "services", s.ID, s); err != nil {
		t.Fatal(err)
	}
	api.call("", "GET", "/services/"+serviceID+"?limit=20&offset=100", nil, 404, nil)
	api.call("worker", "GET", "/services/"+serviceID+"?limit=20&offset=100", nil, 200, nil)
}
