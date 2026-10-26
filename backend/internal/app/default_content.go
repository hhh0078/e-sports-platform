// Package app 初始化受信默认运营资料；新装自动执行一次，已有站点只能显式应用并在同事务备份原公告配置。
package app

import (
	"context"
	"strings"
	"time"

	"jijie/internal/assets"
)

const defaultContentMarker = "platform-default-content-v1"
const defaultContentBackup = "platform-default-content-backup-v1"

// DefaultContentResult 是显式初始化的脱敏结果，不含账号、资金、密钥或原公告正文。
type DefaultContentResult struct {
	Applied                  bool     `json:"applied"`
	AlreadyApplied           bool     `json:"already_applied"`
	Revision                 int64    `json:"revision"`
	BackupID                 string   `json:"backup_id"`
	ArticleIDs               []string `json:"article_ids"`
	AddedSlide               bool     `json:"added_slide"`
	PreservedSlides          int      `json:"preserved_slides"`
	CacheInvalidationPending bool     `json:"cache_invalidation_pending"`
}

// defaultArticles 读取内嵌的已审阅正式正文，默认文章稳定编号不会与客户端随机编号混用。
func defaultArticles(actorID, stamp string) ([]Article, error) {
	definitions := []struct{ id, title, summary, cover string }{
		{"platform-intro-v1", "人人都能接单，也能发布任务", "无需强制挂靠俱乐部，了解如何发布任务、成为可靠队友。", "/static/operations/platform-intro-v1.png"},
		{"newcomer-rules-v1", "新用户必读：交易与接单规则", "先沟通、留凭证，了解验收、退款协商与平台介入。", ""},
	}
	out := []Article{}
	for _, definition := range definitions {
		body, err := assets.DefaultArticleText(definition.id)
		if err != nil {
			return nil, fail(503, "默认运营文章正文尚未完整打包")
		}
		out = append(out, Article{ArticleSummary: ArticleSummary{ID: definition.id, Title: definition.title, Summary: definition.summary, CoverURL: definition.cover, Status: "published", Revision: 1, CreatedAt: stamp, UpdatedAt: stamp, PublishedAt: stamp}, BodyMarkdown: strings.TrimSpace(body), EditorID: actorID})
	}
	return out, nil
}

// applyDefaultContentTx 保留轮播和非空配置，先备份再单调更新公告，所有写入及审核任务与版本标记同事务提交。
func (a *App) applyDefaultContentTx(ctx context.Context, q querier, actor User) (DefaultContentResult, error) {
	result := DefaultContentResult{BackupID: defaultContentBackup, ArticleIDs: []string{"platform-intro-v1", "newcomer-rules-v1"}}
	if _, err := q.Exec(ctx, "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		return result, err
	}
	if prior, err := get[DefaultContentResult](ctx, q, "settings", defaultContentMarker, true); err == nil {
		prior.Applied, prior.AlreadyApplied = false, true
		return prior, nil
	} else if !missing(err) {
		return result, err
	}
	previous, err := get[PublicContentSettings](ctx, q, "settings", publicContentID, true)
	existed := !missing(err)
	if missing(err) {
		previous = defaultPublicContent()
	} else if err != nil {
		return result, err
	}
	for _, slide := range previous.GlobalSlides {
		if slide.ID == "platform-intro-v1" {
			return result, fail(409, "默认轮播标识已被使用且缺少初始化标记，请先人工核对")
		}
	}
	if len(previous.GlobalSlides) >= 12 {
		return result, fail(409, "现有全局轮播已满12项，请先由管理员整理，默认初始化不会删除已有轮播")
	}
	for _, id := range result.ArticleIDs {
		if _, err = get[Article](ctx, q, "articles", id, false); err == nil {
			return result, fail(409, "默认文章编号已存在，拒绝覆盖已有内容")
		} else if !missing(err) {
			return result, err
		}
	}
	if _, err = get[map[string]any](ctx, q, "settings", defaultContentBackup, false); err == nil {
		return result, fail(409, "默认内容备份已存在，拒绝覆盖历史备份")
	} else if !missing(err) {
		return result, err
	}
	if err = putRaw(ctx, q, "settings", defaultContentBackup, map[string]any{"created_at": now(), "existed": existed, "public_content": previous}); err != nil {
		return result, err
	}
	ctx = context.WithValue(ctx, reviewContextKey{}, &reviewRequest{App: a, ActorID: actor.ID})
	stamp := now()
	articles, err := defaultArticles(actor.ID, stamp)
	if err != nil {
		return result, err
	}
	for _, article := range articles {
		_, images, renderErr := renderArticleMarkdown(ctx, q, article.BodyMarkdown, "", actor, true)
		if renderErr != nil {
			return result, renderErr
		}
		article.BodyImages = images
		if err = validateContentImage(ctx, q, article.CoverURL, "", "article_cover", actor); err != nil {
			return result, err
		}
		if err = put(ctx, q, "articles", article.ID, article); err != nil {
			return result, err
		}
	}
	v := previous
	if v.GameSlides == nil {
		v.GameSlides = map[string][]ContentSlide{}
	}
	slide := ContentSlide{ID: "platform-intro-v1", Kind: "image", ImageURL: "/static/operations/platform-intro-v1.png", Labels: []CoverLabel{{ID: "intro-main", Text: "人人都能接单", Color: "#FFFFFF", Background: "transparent", Position: "top-left", FontSize: 24}, {ID: "intro-sub", Text: "每个人，都可以发布任务", Color: "#EBD29B", Background: "transparent", Position: "bottom-right", FontSize: 16}}, Target: &ContentTarget{Kind: "article", ID: "platform-intro-v1"}}
	v.GlobalSlides = append([]ContentSlide{slide}, previous.GlobalSlides...)
	v.Announcement = ContentAnnouncement{ID: "home", Version: previous.Announcement.Version + 1, Enabled: true, Frequency: "once", Mode: "image", ImageURL: "/static/operations/newcomer-rules-v1.png", Title: "新用户必看", Target: &ContentTarget{Kind: "article", ID: "newcomer-rules-v1"}}
	tips := []struct {
		target        *ContentPageTip
		text, article string
	}{
		{&v.PageTips.Player, "先确认服务范围，再与合适的队友一起出发。", "platform-intro-v1"},
		{&v.PageTips.Worker, "无需强制挂靠俱乐部，完善资料后按平台规则接单。", "platform-intro-v1"},
		{&v.PageTips.Orders, "重要约定留在订单聊天里，有争议可申请平台介入。", "newcomer-rules-v1"},
		{&v.PageTips.Me, "资料、订单与钱包，在这里统一查看。", "newcomer-rules-v1"},
	}
	for _, tip := range tips {
		if strings.TrimSpace(tip.target.Text) == "" {
			tip.target.Text = tip.text
			if tip.target.ArticleID == "" {
				tip.target.ArticleID = tip.article
			}
		}
	}
	if strings.TrimSpace(v.ServiceNotice) == "" {
		v.ServiceNotice = "发布前请写明游戏区服、服务范围、价格单位和可预约时间。\n请勿提供账号密码、验证码或使用外挂等违规工具。\n平台押金为自愿服务，不是自由接单的强制条件。\n重要约定请保留在订单聊天中，按约定交付并配合买家验收。"
	}
	if strings.TrimSpace(v.RequestNotice) == "" {
		v.RequestNotice = "请写清游戏区服、任务目标、数量、预算与验收标准。\n下单前先沟通范围，不向他人提供账号密码或验证码。\n重要约定请保留在订单聊天中；退款先协商，无法达成一致可申请平台介入。"
	}
	v.Revision, v.UpdatedAt = previous.Revision+1, stamp
	if err = a.validatePublicContent(ctx, q, &v, previous, actor); err != nil {
		return result, err
	}
	if err = put(ctx, q, "settings", publicContentID, v); err != nil {
		return result, err
	}
	result.Applied, result.AddedSlide, result.Revision, result.PreservedSlides = true, true, v.Revision, len(previous.GlobalSlides)
	if err = putRaw(ctx, q, "settings", defaultContentMarker, result); err != nil {
		return result, err
	}
	err = audit(ctx, q, actor.ID, "default_content.apply", defaultContentMarker, "幂等初始化正式运营图文，保留已有轮播，备份并替换一次性公告，仅补空白提示和须知")
	return result, err
}

// ApplyDefaultContent 为运维显式应用一次默认内容，实时锁定根管理员身份，不创建会话或修改任何交易。
func (a *App) ApplyDefaultContent(ctx context.Context, actorID string) (DefaultContentResult, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return DefaultContentResult{}, err
	}
	defer tx.Rollback(ctx)
	// 与管理端公共配置写入保持先配置事务锁、再操作者行锁的顺序，避免CLI和页面保存互锁。
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		return DefaultContentResult{}, err
	}
	actor, err := lockManagementWriteActor(ctx, tx, actorID)
	if err != nil {
		return DefaultContentResult{}, err
	}
	if !isPlatformRoot(actor) {
		return DefaultContentResult{}, fail(403, "仅有效平台根管理员可初始化默认运营内容")
	}
	result, err := a.applyDefaultContentTx(ctx, tx, actor)
	if err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	if a.Redis != nil {
		flush, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		result.CacheInvalidationPending = a.drainPublicCache(flush, true) != nil
		cancel()
	}
	return result, nil
}
