// Package app 提供独立公共信息读取、管理员配置与当前游戏轮播解析，写入使用标准后置审核事务。
package app

import (
	"context"
	"net/http"
	"reflect"
)

// readPublicContent 只读取固定运营记录，旧项目首次使用回退默认游戏槽。
func readPublicContent(ctx context.Context, q querier) (PublicContentSettings, error) {
	v, err := get[PublicContentSettings](ctx, q, "settings", publicContentID, false)
	if missing(err) {
		return defaultPublicContent(), nil
	}
	if v.GlobalSlides == nil {
		v.GlobalSlides = []ContentSlide{}
	}
	if v.GameSlides == nil {
		v.GameSlides = map[string][]ContentSlide{}
	}
	return v, err
}

// publicContent 返回当前游戏可展示内容，失效目标仅移除行动，不泄露目标私密资料。
func (a *App) publicContent(w http.ResponseWriter, r *http.Request) (any, error) {
	w.Header().Set("Cache-Control", "no-store")
	v, err := publicCached(r.Context(), a, "public-content-source", func() (PublicContentSettings, error) { return readPublicContent(r.Context(), a.DB) })
	if err != nil {
		return nil, err
	}
	g, err := a.shareGame(r.Context(), r.URL.Query().Get("game_id"))
	if err != nil {
		return nil, err
	}
	out := PublicContentDTO{Revision: v.Revision, GameID: g.ID, Slides: []ContentSlide{}, Announcement: v.Announcement, PageTips: v.PageTips, ServiceNotice: v.ServiceNotice, RequestNotice: v.RequestNotice}
	for _, slide := range v.GlobalSlides {
		batch := []ContentSlide{slide}
		if slide.Kind == "game_slot" {
			batch = v.GameSlides[g.ID]
			if len(batch) == 0 {
				batch = []ContentSlide{{ID: "game-default-" + g.ID, Kind: "image", ImageURL: g.CoverURL, Labels: []CoverLabel{}}}
			}
		}
		for _, item := range batch {
			item.Target = a.liveContentTarget(r.Context(), item.Target)
			if item.Labels == nil {
				item.Labels = []CoverLabel{}
			}
			out.Slides = append(out.Slides, item)
		}
	}
	out.Announcement.Target = a.liveContentTarget(r.Context(), out.Announcement.Target)
	for _, tip := range []*ContentPageTip{&out.PageTips.Player, &out.PageTips.Worker, &out.PageTips.Orders, &out.PageTips.Me} {
		if tip.ArticleID != "" && a.liveContentTarget(r.Context(), &ContentTarget{Kind: "article", ID: tip.ArticleID}) == nil {
			tip.ArticleID = ""
		}
	}
	return out, nil
}

// adminPublicContent 仅允许平台管理员获取可编辑公共配置，不返回任何其它settings记录。
func (a *App) adminPublicContent(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return readPublicContent(r.Context(), a.DB)
}

// savePublicContent 按明确顶层字段合并配置，版本冲突整笔拒绝，游戏分组互不清空。
func (a *App) savePublicContent(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Revision      *int64                     `json:"revision"`
		GlobalSlides  *[]ContentSlide            `json:"global_slides"`
		GameSlides    *map[string][]ContentSlide `json:"game_slides"`
		Announcement  *ContentAnnouncement       `json:"announcement"`
		PageTips      *ContentPageTips           `json:"page_tips"`
		ServiceNotice *string                    `json:"service_notice"`
		RequestNotice *string                    `json:"request_notice"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Revision == nil {
		return nil, fail(400, "请提供正在编辑的配置版本")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		return nil, err
	}
	u, err = lockManagementWriteActor(r.Context(), tx, u.ID)
	if err != nil {
		return nil, err
	}
	if u.Role != "admin" {
		return nil, fail(403, "公共信息管理权限已失效")
	}
	previous, err := get[PublicContentSettings](r.Context(), tx, "settings", publicContentID, true)
	if missing(err) {
		previous = defaultPublicContent()
	} else if err != nil {
		return nil, err
	}
	if previous.Revision != *in.Revision {
		return nil, fail(409, "公共信息已更新，请刷新后再保存")
	}
	v := previous
	v.GameSlides = make(map[string][]ContentSlide, len(previous.GameSlides))
	for id, slides := range previous.GameSlides {
		v.GameSlides[id] = slides
	}
	if in.GlobalSlides != nil {
		v.GlobalSlides = *in.GlobalSlides
	}
	if in.GameSlides != nil {
		for id, slides := range *in.GameSlides {
			v.GameSlides[id] = slides
		}
	}
	if in.Announcement != nil {
		v.Announcement = *in.Announcement
		v.Announcement.ID = "home"
		v.Announcement.Version = previous.Announcement.Version
		old, next := previous.Announcement, v.Announcement
		old.Enabled, next.Enabled = false, false
		if !reflect.DeepEqual(old, next) {
			v.Announcement.Version++
		}
	}
	if in.PageTips != nil {
		v.PageTips = *in.PageTips
	}
	if in.ServiceNotice != nil {
		v.ServiceNotice = *in.ServiceNotice
	}
	if in.RequestNotice != nil {
		v.RequestNotice = *in.RequestNotice
	}
	if err = a.validatePublicContent(r.Context(), tx, &v, previous, u); err != nil {
		return nil, err
	}
	v.Revision++
	v.UpdatedAt = now()
	if err = put(r.Context(), tx, "settings", publicContentID, v); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "public_content.save", publicContentID, "更新平台轮播、公告、页面提示与注意事项"); err != nil {
		return nil, err
	}
	return v, tx.Commit(r.Context())
}

// registerPublicContentRoutes 注册公开只读、账号公告已读和管理员维护入口。
func (a *App) registerPublicContentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/public-content", a.wrap(a.publicContent))
	mux.HandleFunc("GET /api/admin/public-content", a.wrap(a.adminPublicContent))
	mux.HandleFunc("PATCH /api/admin/public-content", a.wrap(a.savePublicContent))
	mux.HandleFunc("GET /api/me/announcement-state", a.wrap(a.announcementState))
	mux.HandleFunc("POST /api/me/announcement-ack", a.wrap(a.acknowledgeAnnouncement))
	mux.HandleFunc("GET /api/admin/content-targets", a.wrap(a.searchContentTargets))
	mux.HandleFunc("GET /api/articles", a.wrap(a.publicArticles))
	mux.HandleFunc("GET /api/articles/{id}", a.wrap(a.publicArticle))
	mux.HandleFunc("GET /api/admin/articles", a.wrap(a.adminArticles))
	mux.HandleFunc("GET /api/admin/articles/{id}", a.wrap(a.adminArticle))
	mux.HandleFunc("POST /api/admin/articles", a.wrap(a.saveArticle))
	mux.HandleFunc("PATCH /api/admin/articles/{id}", a.wrap(a.saveArticle))
	mux.HandleFunc("POST /api/admin/articles/preview", a.wrap(a.previewArticle))
}
