// Package app 验证新装默认运营及显式幂等初始化：保留原轮播、备份单公告、空白补值和并发保护。
package app

import (
	"fmt"
	"strings"
	"testing"
)

// TestIntegrationFreshDefaultContent 验证真正的新装路径自动创建两篇正式文章、独立文字首图和一次性透明公告。
func TestIntegrationFreshDefaultContent(t *testing.T) {
	api := newTestAPIWithDefaults(t, true)
	var cfg PublicContentSettings
	api.call("admin", "GET", "/admin/public-content", nil, 200, &cfg)
	if cfg.Revision != 1 || len(cfg.GlobalSlides) != 2 || cfg.GlobalSlides[0].ID != "platform-intro-v1" || cfg.GlobalSlides[1].Kind != "game_slot" {
		t.Fatal("新装默认首图或游戏槽异常")
	}
	if len(cfg.GlobalSlides[0].Labels) != 2 || cfg.GlobalSlides[0].Labels[0].Background != "transparent" || cfg.Announcement.Mode != "image" || cfg.Announcement.Frequency != "once" || cfg.Announcement.Version != 1 {
		t.Fatal("独立文字或新人公告异常")
	}
	for _, id := range []string{"platform-intro-v1", "newcomer-rules-v1"} {
		var article struct {
			ArticleSummary
			BodyHTML string `json:"body_html"`
		}
		api.call("", "GET", "/articles/"+id, nil, 200, &article)
		if article.Status != "published" || !strings.Contains(article.BodyHTML, "<h2>") {
			t.Fatal("默认文章正文未正常渲染", id)
		}
	}
	var tasks int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM moderation_tasks WHERE data->>'target_id' IN ('platform-intro-v1','newcomer-rules-v1','public-content-v1')").Scan(&tasks); err != nil || tasks == 0 {
		t.Fatal("默认正文未进入正常后置审核", err)
	}
	var count int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM users WHERE id LIKE 'system:%'").Scan(&count); err != nil || count != 0 {
		t.Fatal("初始化伪造了系统用户", err)
	}
}

// legacyDefaultContentFixture 构造当前已有站点公开配置，仅操作该测试隔离schema。
func legacyDefaultContentFixture(t *testing.T, api *testAPI, slideCount int) PublicContentSettings {
	t.Helper()
	game := defaultGames()[0]
	cfg := defaultPublicContent()
	cfg.Revision = 7
	cfg.Announcement = ContentAnnouncement{ID: "home", Version: 4, Enabled: true, Frequency: "daily", Mode: "text", Title: "原有公告", Body: "管理员原有公告正文"}
	cfg.ServiceNotice = "保留管理员服务须知"
	cfg.PageTips.Me.Text = "保留个人页提示"
	cfg.GlobalSlides = []ContentSlide{}
	for index := 0; index < slideCount; index++ {
		cfg.GlobalSlides = append(cfg.GlobalSlides, ContentSlide{ID: fmt.Sprintf("legacy-%d", index), Kind: "image", ImageURL: game.CoverURL, Labels: []CoverLabel{}})
	}
	cfg.GameSlides[game.ID] = []ContentSlide{{ID: "old-game-slide", Kind: "image", ImageURL: game.CoverURL, Labels: []CoverLabel{}}}
	if err := putRaw(t.Context(), api.App.DB, "settings", publicContentID, cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// TestIntegrationDefaultContentPreserveAndIdempotent 验证已有站点只显式应用、原公告完整备份、后续人工编辑不被再次覆盖。
func TestIntegrationDefaultContentPreserveAndIdempotent(t *testing.T) {
	api := newTestAPI(t)
	before := legacyDefaultContentFixture(t, api, 2)
	if err := api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := readPublicContent(t.Context(), api.App.DB)
	if unchanged.Revision != before.Revision || unchanged.Announcement.Body != before.Announcement.Body {
		t.Fatal("已有站点启动时自动覆盖了公告")
	}
	result, err := api.App.ApplyDefaultContent(t.Context(), "admin")
	if err != nil || !result.Applied || result.PreservedSlides != 2 {
		t.Fatal("显式初始化失败", result, err)
	}
	current, err := readPublicContent(t.Context(), api.App.DB)
	if err != nil || current.Revision != 8 || len(current.GlobalSlides) != 3 || current.GlobalSlides[1].ID != "legacy-0" || current.Announcement.Version != 5 {
		t.Fatal("没有保留旧轮播或单调更新版本", err)
	}
	if current.ServiceNotice != before.ServiceNotice || current.PageTips.Me.Text != before.PageTips.Me.Text || current.RequestNotice == "" || current.PageTips.Player.Text == "" {
		t.Fatal("覆盖非空值或未补空值")
	}
	for game, slides := range before.GameSlides {
		if len(current.GameSlides[game]) != len(slides) || current.GameSlides[game][0].ID != slides[0].ID {
			t.Fatal("初始化覆盖了原游戏轮播组")
		}
	}
	var backup struct {
		Existed bool                  `json:"existed"`
		Content PublicContentSettings `json:"public_content"`
	}
	backup, err = get[struct {
		Existed bool                  `json:"existed"`
		Content PublicContentSettings `json:"public_content"`
	}](t.Context(), api.App.DB, "settings", defaultContentBackup, false)
	if err != nil || !backup.Existed || backup.Content.Announcement.Body != before.Announcement.Body || backup.Content.Revision != 7 {
		t.Fatal("原公告备份不完整", err)
	}
	current.RequestNotice = "初始化后的管理员修改"
	if err = putRaw(t.Context(), api.App.DB, "settings", publicContentID, current); err != nil {
		t.Fatal(err)
	}
	again, err := api.App.ApplyDefaultContent(t.Context(), "admin")
	stored, _ := readPublicContent(t.Context(), api.App.DB)
	if err != nil || !again.AlreadyApplied || again.Applied || stored.RequestNotice != current.RequestNotice || stored.Revision != 8 {
		t.Fatal("再次执行覆盖了人工内容", err)
	}
}

// TestIntegrationDefaultContentCapacityAndRole 验证容量或角色冲突整笔拒绝，不留下文章、备份或标记。
func TestIntegrationDefaultContentCapacityAndRole(t *testing.T) {
	api := newTestAPI(t)
	legacyDefaultContentFixture(t, api, 12)
	if _, err := api.App.ApplyDefaultContent(t.Context(), "player"); err == nil {
		t.Fatal("普通用户获得初始化权限")
	}
	if _, err := api.App.ApplyDefaultContent(t.Context(), "admin"); err == nil {
		t.Fatal("已满轮播仍被暗删")
	}
	var count int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM articles WHERE id IN ('platform-intro-v1','newcomer-rules-v1')").Scan(&count); err != nil || count != 0 {
		t.Fatal("失败初始化留下默认文章", err)
	}
	for _, id := range []string{defaultContentBackup, defaultContentMarker} {
		if _, err := get[map[string]any](t.Context(), api.App.DB, "settings", id, false); !missing(err) {
			t.Fatal("失败初始化留下备份或标记", id, err)
		}
	}
}

// TestIntegrationDefaultContentConcurrentApply 验证两个真实并发执行只应用一次，不重复首图和文章。
func TestIntegrationDefaultContentConcurrentApply(t *testing.T) {
	api := newTestAPI(t)
	results := make(chan DefaultContentResult, 2)
	errors := make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func() {
			result, err := api.App.ApplyDefaultContent(t.Context(), "admin")
			results <- result
			errors <- err
		}()
	}
	applied, already := 0, 0
	for index := 0; index < 2; index++ {
		result := <-results
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
		if result.Applied {
			applied++
		}
		if result.AlreadyApplied {
			already++
		}
	}
	if applied != 1 || already != 1 {
		t.Fatal("并发初始化不是一次应用", applied, already)
	}
}

// TestIntegrationDefaultContentIDConflict 验证稳定ID被既有内容占用时拒绝整笔应用而不覆盖文章。
func TestIntegrationDefaultContentIDConflict(t *testing.T) {
	api := newTestAPI(t)
	existing := Article{ArticleSummary: ArticleSummary{ID: "platform-intro-v1", Title: "已有文章", Status: "draft", Revision: 9}, BodyMarkdown: "保留现有正文"}
	if err := putRaw(t.Context(), api.App.DB, "articles", existing.ID, existing); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.ApplyDefaultContent(t.Context(), "admin"); err == nil {
		t.Fatal("覆盖了既有稳定ID")
	}
	stored, err := get[Article](t.Context(), api.App.DB, "articles", existing.ID, false)
	if err != nil || stored.BodyMarkdown != existing.BodyMarkdown || stored.Revision != 9 {
		t.Fatal("既有文章被改变", err)
	}
	if _, err = get[map[string]any](t.Context(), api.App.DB, "settings", defaultContentBackup, false); !missing(err) {
		t.Fatal("冲突后留下备份")
	}
}

// TestIntegrationDefaultContentLockOrder 验证CLI等待公共配置锁时不预占管理员行锁，保持与管理页面相同的锁顺序。
func TestIntegrationDefaultContentLockOrder(t *testing.T) {
	api := newTestAPI(t)
	page, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer page.Rollback(t.Context())
	if _, err = page.Exec(t.Context(), "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		t.Fatal(err)
	}
	var blocker uint32
	if err = page.QueryRow(t.Context(), "SELECT pg_backend_pid()").Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, applyErr := api.App.ApplyDefaultContent(t.Context(), "admin"); finished <- applyErr }()
	managementWaitForBlocker(t, api, blocker)
	var id string
	if err = page.QueryRow(t.Context(), "SELECT id FROM users WHERE id='admin' FOR UPDATE NOWAIT").Scan(&id); err != nil {
		t.Fatal("CLI预占操作者行锁，可能与页面保存死锁", err)
	}
	if err = page.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}
