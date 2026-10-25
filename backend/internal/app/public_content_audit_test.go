// Package app 补充公共运营并发版本、深层文字审核与私密图片别名隔离的独立安全回归。
package app

import (
	"strings"
	"sync"
	"testing"
)

// TestIntegrationPublicContentConcurrentRevision 验证同修订并发写只有一个成功，失败方不覆盖其它分节。
func TestIntegrationPublicContentConcurrentRevision(t *testing.T) {
	api := newTestAPI(t)
	type result struct {
		code int
		err  error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for _, field := range []string{"service_notice", "request_notice"} {
		wait.Add(1)
		go func(field string) {
			defer wait.Done()
			code, _, err := api.request("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, field: "并发保存的独立分节"})
			results <- result{code, err}
		}(field)
	}
	wait.Wait()
	close(results)
	codes := map[int]int{}
	for response := range results {
		if response.err != nil {
			t.Fatal(response.err)
		}
		codes[response.code]++
	}
	if codes[200] != 1 || codes[409] != 1 {
		t.Fatalf("并发配置写入未按修订互斥：%v", codes)
	}
	var cfg PublicContentSettings
	api.call("admin", "GET", "/admin/public-content", nil, 200, &cfg)
	if cfg.Revision != 1 || (cfg.ServiceNotice == "") == (cfg.RequestNotice == "") {
		t.Fatal("失败方覆盖了成功分节或错误增加修订")
	}
}

// TestIntegrationAnnouncementAckUpdateRace 验证旧公告确认与新版本发布互斥，任何先后顺序都不能标记新版本已读。
func TestIntegrationAnnouncementAckUpdateRace(t *testing.T) {
	api := newTestAPI(t)
	var cfg PublicContentSettings
	notice := ContentAnnouncement{Enabled: true, Frequency: "once", Mode: "text", Body: "第一版公告"}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "announcement": notice}, 200, &cfg)
	ack := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		code, _, err := api.request("player", "POST", "/me/announcement-ack", map[string]any{"id": "home", "version": 1})
		ack <- code
		errCh <- err
	}()
	notice.Body = "第二版公告"
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "announcement": notice}, 200, &cfg)
	code := <-ack
	if err := <-errCh; err != nil || code != 200 && code != 409 {
		t.Fatalf("旧版本确认异常：状态=%d 错误=%v", code, err)
	}
	var state struct {
		Seen bool `json:"seen"`
	}
	api.call("player", "GET", "/me/announcement-state?version=2", nil, 200, &state)
	if state.Seen {
		t.Fatal("旧公告确认错误标记新版本已读")
	}
	api.call("player", "POST", "/me/announcement-ack", map[string]any{"id": "home", "version": 1}, 409, nil)
}

// TestIntegrationGameSlideReviewPartialMerge 验证深层游戏文字拒绝只修改指定图层，后续局部配置保存不恢复违规字段。
func TestIntegrationGameSlideReviewPartialMerge(t *testing.T) {
	api := newTestAPI(t)
	image := api.uploadCommunityImage("admin", "promotion_banner", "", communityPNG(t, 640, 360), 200)
	// 构造不同游戏的独立图层，固定两条文字以验证相邻字段隔离。
	makeSlide := func(id, text string) ContentSlide {
		return ContentSlide{ID: id, Kind: "image", ImageURL: image, Labels: []CoverLabel{{ID: "caption", Text: text, Color: "#FFFFFF", Background: "transparent", Position: "bottom-left", FontSize: 16}, {ID: "stable", Text: "保留文字", Color: "#FFFFFF", Background: "transparent", Position: "top-left", FontSize: 16}}}
	}
	var cfg PublicContentSettings
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "game_slides": map[string][]ContentSlide{"delta": {makeSlide("slide-a", "待审文字")}, "honor": {makeSlide("slide-b", "其它游戏")}}}, 200, &cfg)
	code := "game_slide.delta." + reviewHash("slide-a") + ".label." + reviewHash("caption")
	task := latestReviewTask(t, api, "settings", publicContentID, code)
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "深层文字拒绝"}, 200, nil)
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "request_notice": "局部新增须知\n保留换行"}, 200, &cfg)
	if cfg.GameSlides["delta"][0].Labels[0].Text != rejectedContentText || cfg.GameSlides["delta"][0].Labels[1].Text != "保留文字" || cfg.GameSlides["honor"][0].Labels[0].Text != "其它游戏" {
		t.Fatal("深层拒绝或局部合并修改了相邻内容")
	}
	var shown PublicContentDTO
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &shown)
	if len(shown.Slides) != 1 || shown.Slides[0].Labels[0].Text != rejectedContentText || shown.RequestNotice != "局部新增须知\n保留换行" {
		t.Fatal("公开响应未使用审核后当前字段")
	}
}

// TestIntegrationArticleImageOwnerAndEvidenceAlias 验证换媒体路径不能公开私密原证据，新增其它管理员图片仍须归属校验。
func TestIntegrationArticleImageOwnerAndEvidenceAlias(t *testing.T) {
	api := newTestAPI(t)
	order := financialOrder(api, "player", "delta-service-1", "dev", "")
	private := api.uploadTestImage("worker", order.ID)
	alias := strings.Replace(strings.Split(private, "?")[0], "/evidence/", "/media/", 1)
	input := map[string]any{"title": "安全图片检查", "summary": "", "body_markdown": "![图片](" + alias + ")", "cover_url": "", "status": "published"}
	api.call("admin", "POST", "/admin/articles", input, 400, nil)
	api.call("admin", "POST", "/admin/articles/preview", map[string]string{"body_markdown": input["body_markdown"].(string)}, 400, nil)
	api.call("admin", "PATCH", "/admin/users/player/deputy", map[string]bool{"enabled": true}, 200, nil)
	image := api.uploadCommunityImage("player", "article_image", "", communityPNG(t, 300, 650), 200)
	input["body_markdown"] = "![他人图片](" + image + ")"
	api.call("admin", "POST", "/admin/articles", input, 403, nil)
	api.call("player", "POST", "/admin/articles", input, 200, nil)
}
