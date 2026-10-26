// Package app 使用真实隔离PG和Redis验收显式游戏文案补齐、配置保留、权限、幂等、审核版本与缓存失效。
package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// captionTaskCount 统计当前隔离schema游戏图层任务，验证重放不会重复入队。
func captionTaskCount(t *testing.T, api *testAPI) int {
	t.Helper()
	var count int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM moderation_tasks WHERE data->>'target_type'='settings' AND data->>'target_id'=$1 AND strpos(data->>'field_code','game_slide.')=1", publicContentID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestIntegrationFillGameCaptionsPreservesConfig 验证仅补无字图、保留原图片和失效旧跳转，标准审核、备份、CAS和即时缓存失效。
func TestIntegrationFillGameCaptionsPreservesConfig(t *testing.T) {
	api := newTestAPI(t)
	article := contentArticleFixture(t, api, "游戏轮播旧跳转")
	delta := defaultGames()[0]
	cfg := defaultPublicContent()
	cfg.Revision = 7
	cfg.GlobalSlides = append([]ContentSlide{{ID: "platform-custom", Kind: "image", ImageURL: "/static/operations/platform-intro-v1.png", Labels: []CoverLabel{{ID: "platform-text", Text: "平台原有文案", Color: "#FFFFFF", Background: "transparent", Position: "top-left", FontSize: 24}}}}, cfg.GlobalSlides...)
	cfg.Announcement = ContentAnnouncement{ID: "home", Version: 9, Enabled: true, Frequency: "once", Mode: "text", Title: "保留平台公告", Body: "原公告正文"}
	cfg.PageTips.Me.Text = "原个人页提示"
	cfg.ServiceNotice = "原服务须知"
	cfg.RequestNotice = "原需求须知"
	target := &ContentTarget{Kind: "article", ID: article.ID}
	cfg.GameSlides["delta"] = []ContentSlide{
		{ID: "custom", Kind: "image", ImageURL: delta.Covers[1], Labels: []CoverLabel{{ID: "custom-text", Text: "已有自定义", Color: "#FFFFFF", Background: "transparent", Position: "middle-left", FontSize: 18}}},
		{ID: "blank", Kind: "image", ImageURL: delta.Covers[1], Labels: []CoverLabel{}, Target: target},
		{ID: "whitespace", Kind: "image", ImageURL: delta.CoverURL, Labels: []CoverLabel{{ID: "empty", Text: " \n "}}},
	}
	cfg.GameSlides["honor"] = []ContentSlide{{ID: "honor-custom", Kind: "image", ImageURL: delta.CoverURL, Labels: []CoverLabel{{ID: "keep", Text: "王者已有文字", Color: "#FFFFFF", Background: "transparent", Position: "top-right", FontSize: 20}}}}
	if err := putRaw(t.Context(), api.App.DB, "settings", publicContentID, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.DB.Exec(t.Context(), `UPDATE articles SET data=jsonb_set(data,'{status}','"hidden"') WHERE id=$1`, article.ID); err != nil {
		t.Fatal(err)
	}
	protected := contentFinanceSnapshot(t, api)
	enablePublicCacheTest(t, api.App)
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, nil)
	generation := api.App.Redis.Get(t.Context(), api.App.publicCacheKeys("")[0]).Val()
	result, err := api.App.FillGameCaptions(t.Context(), "admin")
	if err != nil || !result.Applied || result.FilledSlides != 13 || len(result.FilledGames) != 12 || result.Revision != 8 {
		t.Fatal("补齐数量或版本异常", result, err)
	}
	if api.App.Redis.Get(t.Context(), api.App.publicCacheKeys("")[0]).Val() == generation {
		t.Fatal("提交后没有立即失效")
	}
	stored, err := readPublicContent(t.Context(), api.App.DB)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored.GlobalSlides, cfg.GlobalSlides) || !reflect.DeepEqual(stored.Announcement, cfg.Announcement) || !reflect.DeepEqual(stored.PageTips, cfg.PageTips) || stored.ServiceNotice != cfg.ServiceNotice || stored.RequestNotice != cfg.RequestNotice {
		t.Fatal("补游戏文案改变了平台配置")
	}
	if !reflect.DeepEqual(stored.GameSlides["delta"][0], cfg.GameSlides["delta"][0]) || !reflect.DeepEqual(stored.GameSlides["honor"], cfg.GameSlides["honor"]) {
		t.Fatal("覆盖了已有文案")
	}
	if stored.GameSlides["delta"][1].ImageURL != cfg.GameSlides["delta"][1].ImageURL || !reflect.DeepEqual(stored.GameSlides["delta"][1].Target, target) || stored.GameSlides["delta"][2].ID != "whitespace" {
		t.Fatal("改变了原图片、跳转或顺序")
	}
	if captionTaskCount(t, api) != 26 {
		t.Fatal("补齐未按字段进入正常后置审核")
	}
	backup, err := get[map[string]json.RawMessage](t.Context(), api.App.DB, "settings", gameCaptionBackup, false)
	if err != nil {
		t.Fatal(err)
	}
	var savedBefore PublicContentSettings
	if err = json.Unmarshal(backup["public_content"], &savedBefore); err != nil || !reflect.DeepEqual(savedBefore, cfg) {
		t.Fatal("原配置备份不完整", err)
	}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 7, "game_slides": map[string][]ContentSlide{"delta": {}}}, 409, nil)
	stored.GameSlides["delta"][1].Labels = []CoverLabel{}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": stored.Revision, "game_slides": map[string][]ContentSlide{"delta": stored.GameSlides["delta"]}}, 200, &stored)
	tasks := captionTaskCount(t, api)
	again, err := api.App.FillGameCaptions(t.Context(), "admin")
	after, _ := readPublicContent(t.Context(), api.App.DB)
	if err != nil || !again.AlreadyApplied || again.FilledSlides != 0 || after.Revision != stored.Revision || len(after.GameSlides["delta"][1].Labels) != 0 || captionTaskCount(t, api) != tasks {
		t.Fatal("幂等重放覆盖管理员后续修改或重复审核", err)
	}
	if !reflect.DeepEqual(protected, contentFinanceSnapshot(t, api)) {
		t.Fatal("补齐改变了交易资金")
	}
}

// TestIntegrationFillGameCaptionsPermissionsRollback 验证普通人、副管理员和冻结根管理员均拒绝，候选校验失败不会留下备份和部分写入。
func TestIntegrationFillGameCaptionsPermissionsRollback(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.FillGameCaptions(t.Context(), "player"); err == nil {
		t.Fatal("普通用户可以执行")
	}
	deputy, _ := get[User](t.Context(), api.App.DB, "users", "outsider", false)
	deputy.Role = "admin"
	deputy.PlatformAdminRole = "deputy"
	if err := putRaw(t.Context(), api.App.DB, "users", deputy.ID, deputy); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.FillGameCaptions(t.Context(), deputy.ID); err == nil {
		t.Fatal("副管理员可以执行")
	}
	cfg := defaultPublicContent()
	cfg.GameSlides["delta"] = []ContentSlide{{ID: "bad", Kind: "image", ImageURL: "https://invalid.example/private.png", Labels: []CoverLabel{}}}
	if err := putRaw(t.Context(), api.App.DB, "settings", publicContentID, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.FillGameCaptions(t.Context(), "admin"); err == nil {
		t.Fatal("无效源图没有整笔拒绝")
	}
	after, _ := readPublicContent(t.Context(), api.App.DB)
	if !reflect.DeepEqual(after, cfg) || captionTaskCount(t, api) != 0 {
		t.Fatal("失败留下部分配置或审核任务")
	}
	for _, id := range []string{gameCaptionBackup, gameCaptionMarker} {
		if _, err := get[map[string]any](t.Context(), api.App.DB, "settings", id, false); !missing(err) {
			t.Fatal("失败留下备份或标记", id, err)
		}
	}
	admin, _ := get[User](t.Context(), api.App.DB, "users", "admin", false)
	admin.Status = "frozen"
	if err := putRaw(t.Context(), api.App.DB, "users", admin.ID, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.FillGameCaptions(t.Context(), "admin"); err == nil {
		t.Fatal("冻结根管理员可以执行")
	}
}

// TestIntegrationFillGameCaptionsRejectedGuard 验证拒绝过的当前游戏分组不会被补齐或通过空分组回退绕过。
func TestIntegrationFillGameCaptionsRejectedGuard(t *testing.T) {
	api := newTestAPI(t)
	game := defaultGames()[0]
	slide := ContentSlide{ID: "reviewed", Kind: "image", ImageURL: game.CoverURL, Labels: []CoverLabel{{ID: "line", Text: "等待审核的文字", Color: "#FFFFFF", Background: "transparent", Position: "top-left", FontSize: 24}}}
	var cfg PublicContentSettings
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": 0, "game_slides": map[string][]ContentSlide{"delta": {slide}}}, 200, &cfg)
	task := latestReviewTask(t, api, "settings", publicContentID, "game_slide.delta."+reviewHash(slide.ID)+".label."+reviewHash("line"))
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "拒绝此字段"}, 200, nil)
	cfg, _ = readPublicContent(t.Context(), api.App.DB)
	cfg.GameSlides["delta"][0].Labels = []CoverLabel{}
	if err := putRaw(t.Context(), api.App.DB, "settings", publicContentID, cfg); err != nil {
		t.Fatal(err)
	}
	result, err := api.App.FillGameCaptions(t.Context(), "admin")
	if err != nil || !contains(result.RejectedGames, "delta") || result.FilledSlides != 12 {
		t.Fatal("没有跳过当前拒绝分组", result, err)
	}
	after, _ := readPublicContent(t.Context(), api.App.DB)
	if len(after.GameSlides["delta"][0].Labels) != 0 {
		t.Fatal("默认文字覆盖拒绝状态")
	}
	after.GameSlides["delta"] = []ContentSlide{}
	if err = putRaw(t.Context(), api.App.DB, "settings", publicContentID, after); err != nil {
		t.Fatal(err)
	}
	var public PublicContentDTO
	api.call("", "GET", "/public-content?game_id=delta", nil, 200, &public)
	if len(public.Slides) != 1 || len(public.Slides[0].Labels) != 0 {
		t.Fatal("空分组回退绕过了拒绝")
	}
}

// TestIntegrationFillGameCaptionsLateReview 验证默认字段可正常编辑及审核，迟到旧决定不能覆盖新版，重放不恢复拒绝文字。
func TestIntegrationFillGameCaptionsLateReview(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.FillGameCaptions(t.Context(), "admin"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := readPublicContent(t.Context(), api.App.DB)
	slide := cfg.GameSlides["delta"][0]
	code := "game_slide.delta." + reviewHash(slide.ID) + ".label." + reviewHash("game-caption-main")
	old := latestReviewTask(t, api, "settings", publicContentID, code)
	slide.Labels[0].Text = "管理员更新的文案"
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "game_slides": map[string][]ContentSlide{"delta": {slide}}}, 200, &cfg)
	api.call("admin", "POST", "/admin/moderation/"+old.ID+"/decision", map[string]string{"decision": "reject", "reason": "迟到旧结果"}, 409, nil)
	current := latestReviewTask(t, api, "settings", publicContentID, code)
	api.call("admin", "POST", "/admin/moderation/"+current.ID+"/decision", map[string]string{"decision": "reject", "reason": "当前文字拒绝"}, 200, nil)
	if _, err := api.App.FillGameCaptions(t.Context(), "admin"); err != nil {
		t.Fatal(err)
	}
	after, _ := readPublicContent(t.Context(), api.App.DB)
	if after.GameSlides["delta"][0].Labels[0].Text != rejectedContentText {
		t.Fatal("幂等执行恢复了拒绝文字")
	}
}

// TestIntegrationFillGameCaptionsConcurrent 验证两个并发执行只应用一次，未重复加图和审核字段。
func TestIntegrationFillGameCaptionsConcurrent(t *testing.T) {
	api := newTestAPI(t)
	type outcome struct {
		result GameCaptionFillResult
		err    error
	}
	done := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() { r, e := api.App.FillGameCaptions(t.Context(), "admin"); done <- outcome{r, e} }()
	}
	applied, already := 0, 0
	for i := 0; i < 2; i++ {
		r := <-done
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.result.Applied {
			applied++
		}
		if r.result.AlreadyApplied {
			already++
		}
	}
	if applied != 1 || already != 1 || captionTaskCount(t, api) != 26 {
		t.Fatal("并发执行重复写入", applied, already)
	}
	cfg, _ := readPublicContent(t.Context(), api.App.DB)
	for _, slides := range cfg.GameSlides {
		if len(slides) != 1 {
			t.Fatal("重复加图")
		}
	}
}

// TestIntegrationFillGameCaptionsActorRevoked 验证真实锁等待期间撤权后旧执行不能继续写配置。
func TestIntegrationFillGameCaptionsActorRevoked(t *testing.T) {
	api := newTestAPI(t)
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if _, err = tx.Exec(t.Context(), "SELECT id FROM users WHERE id='admin' FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	var pid uint32
	if err = tx.QueryRow(t.Context(), "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, e := api.App.FillGameCaptions(t.Context(), "admin"); done <- e }()
	managementWaitForBlocker(t, api, pid)
	if _, err = tx.Exec(t.Context(), `UPDATE users SET data=jsonb_set(data,'{role}','"user"') WHERE id='admin'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err == nil {
		t.Fatal("在途撤权后仍应用了文案")
	}
	if _, err = get[map[string]any](t.Context(), api.App.DB, "settings", gameCaptionMarker, false); !missing(err) {
		t.Fatal("撤权失败后留下标记")
	}
}

// TestIntegrationFreshGameCaptionsSeed 验证全新安装默认GameSlides正常审核，重启和幂等命令不覆盖后续清空。
func TestIntegrationFreshGameCaptionsSeed(t *testing.T) {
	api := newTestAPIWithDefaults(t, true)
	cfg, _ := readPublicContent(t.Context(), api.App.DB)
	if len(cfg.GameSlides) != 13 || captionTaskCount(t, api) != 26 {
		t.Fatal("新装没有正常创建十三组待审文案")
	}
	for _, game := range defaultGames() {
		slides := cfg.GameSlides[game.ID]
		if len(slides) != 1 || slides[0].ImageURL != game.CoverURL || !reflect.DeepEqual(slides[0].Labels, GameCarouselLabels(game)) {
			t.Fatal("新装游戏图文不匹配", game.ID)
		}
	}
	cfg.GameSlides["delta"][0].Labels = []CoverLabel{}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "game_slides": map[string][]ContentSlide{"delta": cfg.GameSlides["delta"]}}, 200, &cfg)
	count := captionTaskCount(t, api)
	if err := api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	result, err := api.App.FillGameCaptions(t.Context(), "admin")
	after, _ := readPublicContent(t.Context(), api.App.DB)
	if err != nil || !result.AlreadyApplied || len(after.GameSlides["delta"][0].Labels) != 0 || after.Revision != cfg.Revision || captionTaskCount(t, api) != count {
		t.Fatal("重启或重复执行覆盖了人工修改", err)
	}
}

// currentUploadedGameCoverFixture 在隔离库配置无运营上传用途的真实游戏封面，返回源游戏和原件供回归核对。
func currentUploadedGameCoverFixture(t *testing.T, api *testAPI) (Game, Evidence) {
	t.Helper()
	raw := communityPNG(t, 640, 360)
	image := Evidence{ID: newID(), OwnerID: "admin", Public: true, MIME: "image/png", Data: raw, SHA256: reviewHash(string(raw)), Moderation: "manual", CreatedAt: now()}
	if err := putRaw(t.Context(), api.App.DB, "evidence", image.ID, image); err != nil {
		t.Fatal(err)
	}
	game, _ := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	game.CoverURL = "/api/media/" + image.ID
	game.Covers = append(game.Covers, game.CoverURL)
	if err := putRaw(t.Context(), api.App.DB, "games", game.ID, game); err != nil {
		t.Fatal(err)
	}
	return game, image
}

// TestIntegrationFillGameCaptionsCurrentUploadedCover 验证只引用当前真实公开游戏封面，不要求把旧封面伪装成运营上传用途。
func TestIntegrationFillGameCaptionsCurrentUploadedCover(t *testing.T) {
	api := newTestAPI(t)
	game, image := currentUploadedGameCoverFixture(t, api)
	if _, err := api.App.FillGameCaptions(t.Context(), "admin"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := readPublicContent(t.Context(), api.App.DB)
	if cfg.GameSlides["delta"][0].ImageURL != game.CoverURL || len(cfg.GameSlides["delta"][0].Labels) != 2 {
		t.Fatal("改变了实际游戏封面")
	}
	stored, err := get[Evidence](t.Context(), api.App.DB, "evidence", image.ID, false)
	if err != nil || stored.SHA256 != image.SHA256 || !strings.EqualFold(stored.MIME, image.MIME) || !reflect.DeepEqual(stored.Data, image.Data) {
		t.Fatal("改写了封面原件", err)
	}
}

// TestIntegrationFillGameCaptionsUploadedCoverRemainsEditable 验证补齐后普通须知保存和图层编辑不被图片用途阻断，跨游戏和平台图仍严格拒绝。
func TestIntegrationFillGameCaptionsUploadedCoverRemainsEditable(t *testing.T) {
	api := newTestAPI(t)
	game, image := currentUploadedGameCoverFixture(t, api)
	if _, err := api.App.FillGameCaptions(t.Context(), "admin"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := readPublicContent(t.Context(), api.App.DB)
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "service_notice": "只编辑服务须知"}, 200, &cfg)
	slide := cfg.GameSlides[game.ID][0]
	slide.Labels[0].Text = "更新游戏轮播文案"
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "game_slides": map[string][]ContentSlide{game.ID: {slide}}}, 200, &cfg)
	if cfg.GameSlides[game.ID][0].ImageURL != game.CoverURL || cfg.GameSlides[game.ID][0].Labels[0].Text != slide.Labels[0].Text {
		t.Fatal("正常编辑丢失当前游戏封面或文字")
	}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "game_slides": map[string][]ContentSlide{"honor": {slide}}}, 400, nil)
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "global_slides": []ContentSlide{slide}}, 400, nil)
	game.CoverURL = defaultGames()[0].CoverURL
	api.call("admin", "PATCH", "/admin/games/"+game.ID, game, 200, nil)
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "service_notice": "游戏换封面后仍可编辑须知"}, 200, &cfg)
	if cfg.GameSlides[game.ID][0].ImageURL != slide.ImageURL {
		t.Fatal("更换游戏封面偷偷改写了原有轮播")
	}
	newSlide := slide
	newSlide.ID = "new-legacy-image-reference"
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "game_slides": map[string][]ContentSlide{game.ID: {newSlide}}}, 400, nil)
	image.Moderation = "rejected"
	if err := putRaw(t.Context(), api.App.DB, "evidence", image.ID, image); err != nil {
		t.Fatal(err)
	}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "service_notice": "不能借旧引用绕过撤销"}, 400, nil)
}

// TestIntegrationDefaultContentCurrentUploadedGameCover 验证默认运营首次初始化兼容当前托管游戏封面，后续普通配置保存仍可用。
func TestIntegrationDefaultContentCurrentUploadedGameCover(t *testing.T) {
	api := newTestAPI(t)
	game, _ := currentUploadedGameCoverFixture(t, api)
	if _, err := api.App.ApplyDefaultContent(t.Context(), "admin"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := readPublicContent(t.Context(), api.App.DB)
	if cfg.GameSlides[game.ID][0].ImageURL != game.CoverURL || len(cfg.GameSlides[game.ID][0].Labels) != 2 {
		t.Fatal("默认初始化没有保留真实游戏封面")
	}
	api.call("admin", "PATCH", "/admin/public-content", map[string]any{"revision": cfg.Revision, "request_notice": "初始化后编辑需求须知"}, 200, &cfg)
}
