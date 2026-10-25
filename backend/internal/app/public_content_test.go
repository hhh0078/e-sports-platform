// Package app 验证公共运营配置的完整合并、图层排序、公告跨设备已读及审核版本保护。
package app

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// contentArticleFixture 通过真实管理员接口发布一篇文章，供跳转和页面提示引用。
func contentArticleFixture(t *testing.T, api *testAPI, title string) Article {
	t.Helper()
	var article Article
	api.call("admin", "POST", "/admin/articles", map[string]any{"title": title, "summary": "文章摘要", "body_markdown": "# 正常文章\n\n一起保持良好游戏环境。", "cover_url": "", "status": "published"}, 200, &article)
	return article
}

// contentFinanceSnapshot 只对资金、订单和原交易证据求摘要，验证运营写入不越过业务边界。
func contentFinanceSnapshot(t *testing.T, api *testAPI) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range tables {
		if !strings.HasPrefix(table, "wallet") && !strings.HasPrefix(table, "fund_") && !strings.HasPrefix(table, "order_") && !contains([]string{"orders", "messages", "reviews"}, table) {
			continue
		}
		rows, err := list[map[string]any](t.Context(), api.App.DB, table, "ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		out[table] = reviewHash(string(raw))
	}
	return out
}

// TestIntegrationPublicContentConfiguration 验证独立图层、全局游戏展开、权限、合并与注意事项审核。
func TestIntegrationPublicContentConfiguration(t *testing.T) {
	api := newTestAPI(t)
	var initial PublicContentDTO
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &initial)
	if len(initial.Slides) != 1 || initial.Slides[0].ImageURL != "/static/covers/official/delta-1.jpg" {
		t.Fatal("默认游戏槽未保留当前游戏封面")
	}
	api.call("", "GET", "/public-content?game_id=missing-game", nil, 404, nil)
	api.call("", "GET", "/admin/public-content", nil, 401, nil)
	api.call("player", "PATCH", "/admin/public-content", map[string]any{"revision": 0}, 403, nil)
	article := contentArticleFixture(t, api, "安全服务须知")
	protected := contentFinanceSnapshot(t, api)
	beforeGame, _ := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	banner := api.uploadCommunityImage("admin", "promotion_banner", "", communityPNG(t, 640, 360), 200)
	api.uploadCommunityImage("player", "promotion_banner", "", communityPNG(t, 640, 360), 403)
	api.uploadCommunityImage("admin", "promotion_banner", "", communityPNG(t, 400, 900), 400)
	noticeImage := api.uploadCommunityImage("admin", "announcement_image", "", communityPNG(t, 200, 700), 200)
	labels := []CoverLabel{{ID: "line-a", Text: "独立轮播字", Color: "#FFFFFF", Background: "#101614", Position: "bottom-left", FontSize: 18}}
	image := ContentSlide{ID: "platform", Kind: "image", ImageURL: banner, Labels: labels, Target: &ContentTarget{Kind: "article", ID: article.ID}}
	deltaA := image
	deltaA.ID = "delta-a"
	deltaB := image
	deltaB.ID = "delta-b"
	var saved PublicContentSettings
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{
		"revision": 0, "global_slides": []ContentSlide{image, {ID: "game", Kind: "game_slot"}},
		"game_slides":    map[string][]ContentSlide{"delta": {deltaA, deltaB}},
		"announcement":   ContentAnnouncement{Enabled: true, Frequency: "once", Mode: "image", ImageURL: noticeImage, Title: "平台公告", Body: "第一行\n第二行"},
		"page_tips":      ContentPageTips{Player: ContentPageTip{Text: "玩家提示\n保留换行", ArticleID: article.ID}},
		"service_notice": "服务前先沟通\n请勿发送密码", "request_notice": "说明游戏区服\n约定验收标准",
	}, 200, &saved)
	if saved.Revision != 1 || saved.Announcement.Version != 1 || saved.Announcement.ID != "home" {
		t.Fatalf("配置或公告版本异常：%+v", saved)
	}
	var shown PublicContentDTO
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &shown)
	if len(shown.Slides) != 3 || shown.Slides[0].ID != "platform" || shown.Slides[1].ID != "delta-a" || shown.Slides[2].ID != "delta-b" || !reflect.DeepEqual(shown.Slides[1].Labels, labels) || shown.ServiceNotice != "服务前先沟通\n请勿发送密码" || shown.PageTips.Player.ArticleID != article.ID {
		t.Fatalf("展开排序、独立图层或换行异常：%+v", shown)
	}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "service_notice": "陈旧草稿"}, 409, nil)
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 1, "global_slides": []ContentSlide{{ID: "a", Kind: "game_slot"}, {ID: "b", Kind: "game_slot"}}}, 400, nil)
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 1, "game_slides": map[string][]ContentSlide{"honor": {image}}}, 200, &saved)
	if len(saved.GameSlides["delta"]) != 2 || len(saved.GameSlides["honor"]) != 1 || saved.Announcement.Version != 1 {
		t.Fatal("保存单个游戏清空了其它游戏或误改公告版本")
	}
	task := latestReviewTask(t, api, "settings", publicContentID, "service_notice")
	if task.Status != "manual_pending" {
		t.Fatal("注意事项未进入人工兜底审核")
	}
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "测试拒绝处理"}, 200, nil)
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &shown)
	if shown.ServiceNotice != rejectedContentText {
		t.Fatal("被拒注意事项未替换")
	}
	updatedGame, _ := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if !reflect.DeepEqual(beforeGame.Covers, updatedGame.Covers) || !reflect.DeepEqual(protected, contentFinanceSnapshot(t, api)) {
		t.Fatal("运营配置修改了服务封面池或资金订单")
	}
	var raw map[string]any
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &raw)
	encoded, _ := json.Marshal(raw)
	for _, private := range []string{"secret", "openid", "editor_id", "game_slides", "global_slides"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("公开DTO混入不需要的配置：%s", private)
		}
	}
}

// TestIntegrationAnnouncementReceipts 验证已读按账号与版本隔离，重复确认幂等，版本变化重新未读。
func TestIntegrationAnnouncementReceipts(t *testing.T) {
	api := newTestAPI(t)
	var cfg PublicContentSettings
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "announcement": ContentAnnouncement{Enabled: true, Frequency: "once", Mode: "text", Title: "新公告", Body: "公告第一版"}}, 200, &cfg)
	var state struct {
		Seen bool `json:"seen"`
	}
	path := fmt.Sprintf("/me/announcement-state?version=%d", cfg.Announcement.Version)
	api.call("", "GET", path, nil, 401, nil)
	api.call("player", "GET", path, nil, 200, &state)
	if state.Seen {
		t.Fatal("读取状态不能提前已读")
	}
	for index := 0; index < 2; index++ {
		api.call("player", "POST", "/me/announcement-ack", map[string]any{"id": "home", "version": 1}, 200, nil)
	}
	api.call("player", "GET", path, nil, 200, &state)
	if !state.Seen {
		t.Fatal("同账号另一设备未复用服务器已读")
	}
	api.call("worker", "GET", path, nil, 200, &state)
	if state.Seen {
		t.Fatal("公告已读串到其它账号")
	}
	notice := cfg.Announcement
	notice.Enabled = false
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "announcement": notice}, 200, &cfg)
	if cfg.Announcement.Version != 1 {
		t.Fatal("仅开关公告不应重复发布版本")
	}
	notice = cfg.Announcement
	notice.Body = "公告第二版"
	notice.Enabled = true
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "announcement": notice}, 200, &cfg)
	api.call("player", "POST", "/me/announcement-ack", map[string]any{"id": "home", "version": 1}, 409, nil)
	api.call("player", "GET", "/me/announcement-state?version=2", nil, 200, &state)
	if state.Seen {
		t.Fatal("新版本应重新未读")
	}
	var count int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM announcement_receipts").Scan(&count)
	if count != 1 {
		t.Fatalf("重复已读不幂等：%d", count)
	}
}
