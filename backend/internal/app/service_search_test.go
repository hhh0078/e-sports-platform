// Package app 使用超过一百条的商品夹具验证服务端排序分页及公开可见性边界。
package app

import (
	"fmt"
	"net/url"
	"testing"
)

// TestIntegrationServiceSortBeforePagination 验证最便宜和成交最高商品即使不在推荐首百条也能正确排到首位。
func TestIntegrationServiceSortBeforePagination(t *testing.T) {
	api := newTestAPI(t)
	base, err := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	if err != nil {
		t.Fatal(err)
	}
	base.Title, base.Description, base.ClubID = "排序验证服务", "排序验证夹具，用于测试过滤、分页和真实目录的隔离。", ""
	base.CreatedAt = "2026-10-18T00:00:00Z"
	for i := 0; i < 130; i++ {
		s := base
		s.ID = fmt.Sprintf("sort-%03d", i)
		s.PriceCents, s.CompletedCount = int64(100+i/5), 130-i/5
		if err = put(t.Context(), api.App.DB, "services", s.ID, s); err != nil {
			t.Fatal(err)
		}
	}
	// 隐藏商品故意使用更低价格和更多成交，必须在 LIMIT 之前排除。
	for _, kind := range []string{"paused", "frozen", "disabled", "orphan"} {
		for i := 0; i < 20; i++ {
			s := base
			s.ID, s.PriceCents, s.CompletedCount = fmt.Sprintf("hidden-%s-%03d", kind, i), 1, 999
			s.CreatedAt = "2026-10-18T01:00:00Z"
			switch kind {
			case "paused":
				s.Status = "paused"
			case "frozen":
				s.ProviderID = "outsider"
			case "disabled":
				s.GameID = "honor"
			case "orphan":
				s.GameID = "missing-game"
			}
			if err = put(t.Context(), api.App.DB, "services", s.ID, s); err != nil {
				t.Fatal(err)
			}
		}
	}
	api.call("admin", "PATCH", "/admin/users/outsider", map[string]string{"status": "frozen"}, 200, nil)
	honor, err := get[Game](t.Context(), api.App.DB, "games", "honor", false)
	if err != nil {
		t.Fatal(err)
	}
	honor.Enabled = false
	api.call("admin", "PATCH", "/admin/games/honor", honor, 200, nil)
	for _, sortName := range []string{"recommended", "price", "completed"} {
		t.Run(sortName, func(t *testing.T) {
			seen := map[string]bool{}
			for offset := 0; offset < 144; offset += 24 {
				var page []Service
				api.call("", "GET", fmt.Sprintf("/services?q=%s&sort=%s&limit=24&offset=%d", url.QueryEscape("排序验证"), sortName, offset), nil, 200, &page)
				want := 24
				if offset+want > 130 {
					want = 130 - offset
				}
				if len(page) != want {
					t.Fatalf("%s offset=%d 期望%d项，实际%d", sortName, offset, want, len(page))
				}
				for i, s := range page {
					position := offset + i
					if sortName == "recommended" {
						position = 129 - position
					}
					if s.ID != fmt.Sprintf("sort-%03d", position) || seen[s.ID] {
						t.Fatalf("排序不正确、重复或泄漏隐藏商品：%s %s", sortName, s.ID)
					}
					seen[s.ID] = true
				}
			}
			if len(seen) != 130 {
				t.Fatalf("跨页应无重复无丢失，实际 %d", len(seen))
			}
		})
	}
	var matches []Service
	api.call("", "GET", "/services?sort=price&game_id=delta&category="+url.QueryEscape(base.Category)+"&q="+url.QueryEscape("排序验证")+"&limit=1", nil, 200, &matches)
	if len(matches) != 1 || matches[0].ID != "sort-000" {
		t.Fatal("游戏与分类过滤必须先于排序分页")
	}
	api.call("", "GET", "/services?q="+url.QueryEscape("' OR 1=1 --"), nil, 200, &matches)
	if len(matches) != 0 {
		t.Fatal("SQL 关键字必须按普通搜索文本处理")
	}
	api.call("", "GET", "/services?sort=unknown", nil, 400, nil)
	api.call("", "GET", "/services?mine=1", nil, 401, nil)
	api.call("player", "GET", "/services?mine=1&q="+url.QueryEscape("排序验证"), nil, 200, &matches)
	if len(matches) != 0 {
		t.Fatal("我的服务不能泄漏其他人的发布记录")
	}
	// 通过分类及真实标题检索本人已暂停服务，状态不应按公开可见性隐藏。
	api.call("worker", "GET", "/services?mine=1&q="+url.QueryEscape("排序验证")+"&sort=recommended&limit=100", nil, 200, &matches)
	foundPaused := false
	for _, s := range matches {
		foundPaused = foundPaused || s.Status == "paused"
	}
	if !foundPaused {
		t.Fatal("本人暂停的服务应可在我的发布中找到")
	}
}
