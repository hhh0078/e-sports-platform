// Package app 验证旧运营跳转下架后不阻断无关保存，新增、改目标或跨对象复用仍须通过公开资格校验。
package app

import (
	"reflect"
	"testing"
)

// staleContentFixture 通过真实管理接口建立平台、游戏、公告及提示四类跳转，随后将目标服务和文章下架。
func staleContentFixture(t *testing.T) (*testAPI, PublicContentSettings, Article) {
	t.Helper()
	api := newTestAPI(t)
	article := contentArticleFixture(t, api, "即将下架的文章")
	banner := api.uploadCommunityImage("admin", "promotion_banner", "", communityPNG(t, 640, 360), 200)
	var cfg PublicContentSettings
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{
		"revision":      0,
		"global_slides": []ContentSlide{{ID: "platform-a", Kind: "image", ImageURL: banner, Target: &ContentTarget{Kind: "service", ID: "delta-service-1"}}, {ID: "game", Kind: "game_slot"}},
		"game_slides":   map[string][]ContentSlide{"delta": {{ID: "delta-a", Kind: "image", ImageURL: banner, Target: &ContentTarget{Kind: "article", ID: article.ID}}}},
		"announcement":  ContentAnnouncement{Enabled: true, Mode: "text", Frequency: "once", Body: "不含失效链接的正常公告正文", Target: &ContentTarget{Kind: "service", ID: "delta-service-1"}},
		"page_tips":     ContentPageTips{Player: ContentPageTip{Text: "阅读文章", ArticleID: article.ID}},
	}, 200, &cfg)
	api.call("admin", "PATCH", "/admin/services/delta-service-1", map[string]string{"status": "paused"}, 200, nil)
	api.call("admin", "PATCH", "/admin/articles/"+article.ID, map[string]any{"revision": article.Revision, "title": article.Title, "summary": article.Summary, "body_markdown": article.BodyMarkdown, "cover_url": article.CoverURL, "status": "hidden"}, 200, nil)
	return api, cfg, article
}

// TestIntegrationStaleContentTargetsDoNotBlockOtherSections 验证只改须知也能保存，旧跳转自动清除，图文、版本已读及资金不被误改。
func TestIntegrationStaleContentTargetsDoNotBlockOtherSections(t *testing.T) {
	api, cfg, _ := staleContentFixture(t)
	protected := contentFinanceSnapshot(t, api)
	var shown PublicContentDTO
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &shown)
	if shown.Slides[0].Target != nil || shown.Slides[1].Target != nil || shown.Announcement.Target != nil || shown.PageTips.Player.ArticleID != "" {
		t.Fatal("下架目标仍经公开投影返回跳转")
	}
	announcementVersion := cfg.Announcement.Version
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "service_notice": "独立修改服务须知\n第二行保留"}, 200, &cfg)
	if cfg.GlobalSlides[0].Target != nil || cfg.GameSlides["delta"][0].Target != nil || cfg.Announcement.Target != nil || cfg.PageTips.Player.ArticleID != "" {
		t.Fatal("同一字段旧失效跳转没有自动撤销")
	}
	if cfg.ServiceNotice != "独立修改服务须知\n第二行保留" || cfg.Announcement.Body != "不含失效链接的正常公告正文" || cfg.PageTips.Player.Text != "阅读文章" || cfg.Announcement.Version != announcementVersion || len(cfg.GlobalSlides) != 2 || len(cfg.GameSlides["delta"]) != 1 {
		t.Fatal("无关保存误改文字、排序或把自动撤链当成新公告")
	}
	if !reflect.DeepEqual(protected, contentFinanceSnapshot(t, api)) {
		t.Fatal("清理失效运营跳转改变了交易和资金")
	}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "request_notice": "后续仍可继续保存"}, 200, &cfg)
}

// TestIntegrationStaleTargetsCannotBeCopiedToNewObjects 验证同轮播ID重排允许撤链，复制ID、分组、字段或变更目标不能复用旧资格。
func TestIntegrationStaleTargetsCannotBeCopiedToNewObjects(t *testing.T) {
	api, cfg, article := staleContentFixture(t)
	copySlide := cfg.GlobalSlides[0]
	copySlide.ID = "new-platform"
	for _, patch := range []map[string]any{
		{"global_slides": []ContentSlide{copySlide}},
		{"game_slides": map[string][]ContentSlide{"honor": {cfg.GlobalSlides[0]}}},
		{"page_tips": ContentPageTips{Worker: ContentPageTip{Text: "复制到不同页面", ArticleID: article.ID}}},
		{"announcement": ContentAnnouncement{Mode: "text", Frequency: "once", Body: "改变公告跳转", Target: &ContentTarget{Kind: "article", ID: article.ID}}},
	} {
		patch["revision"] = cfg.Revision
		api.call("admin", "PATCH", "/admin/public-content", patch, 404, nil)
	}
	var unchanged PublicContentSettings
	api.call("admin", "GET", "/admin/public-content", nil, 200, &unchanged)
	if unchanged.Revision != cfg.Revision || unchanged.GlobalSlides[0].Target == nil || unchanged.GameSlides["delta"][0].Target == nil {
		t.Fatal("被拒的新目标保存提前清理或提交了原记录")
	}
	platform := cfg.GlobalSlides[0]
	platform.Labels = []CoverLabel{{ID: "caption", Text: "保留运营文字", Color: "#FFFFFF", Background: "transparent", Position: "bottom-left", FontSize: 16}}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "global_slides": []ContentSlide{cfg.GlobalSlides[1], platform}}, 200, &cfg)
	if cfg.GlobalSlides[1].Target != nil || cfg.GlobalSlides[1].Labels[0].Text != "保留运营文字" || cfg.GlobalSlides[0].Kind != "game_slot" {
		t.Fatal("同一轮播ID重排没有保留独立文字或撤销旧跳转")
	}
	platform.Target = &ContentTarget{Kind: "service", ID: "delta-service-1"}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "global_slides": []ContentSlide{platform}}, 404, nil)
}
