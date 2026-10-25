// Package app 提供管理员Markdown文章维护与访客发布内容查询，草稿隐藏、图文审核和安全渲染相互独立。
package app

import (
	"net/http"
	"strings"
)

// ArticleSummary 为列表及公开详情提供稳定摘要，不包含编辑账号和Markdown原稿。
type ArticleSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	CoverURL    string `json:"cover_url"`
	Status      string `json:"status"`
	Revision    int64  `json:"revision"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	PublishedAt string `json:"published_at"`
}

// Article 保留服务端编辑来源及正文媒体引用，用于字段级后置审核。
type Article struct {
	ArticleSummary
	BodyMarkdown string   `json:"body_markdown"`
	BodyImages   []string `json:"body_images"`
	EditorID     string   `json:"editor_id"`
}

// adminArticleView 限制管理员响应为契约字段，原稿可编辑但不混入额外私密来源。
func adminArticleView(v Article) any {
	return struct {
		ArticleSummary
		BodyMarkdown string `json:"body_markdown"`
	}{v.ArticleSummary, v.BodyMarkdown}
}

// articlePage 在完整文章集合中按标题编号搜索后分页，公开集合始终只含已发布文章。
func (a *App) articlePage(r *http.Request, management bool) (any, error) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	status := r.URL.Query().Get("status")
	if !length(keyword, 0, 100) || status != "" && !contains([]string{"draft", "published", "hidden"}, status) {
		return nil, fail(400, "文章搜索或状态不正确")
	}
	if !management {
		status = "published"
	}
	where := "WHERE ($1='' OR data->>'status'=$1) AND ($2='' OR strpos(lower(concat_ws(' ',id,data->>'title')),lower($2))>0)"
	var total int
	if err := a.DB.QueryRow(r.Context(), "SELECT count(*) FROM articles "+where, status, keyword).Scan(&total); err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	items, err := list[ArticleSummary](r.Context(), a.DB, "articles", where+" ORDER BY data->>'created_at' DESC,id DESC LIMIT $3 OFFSET $4", status, keyword, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total}, nil
}

// publicArticles 允许访客分页阅读已发布文章，管理员草稿不会通过查询参数公开。
func (a *App) publicArticles(w http.ResponseWriter, r *http.Request) (any, error) {
	w.Header().Set("Cache-Control", "no-store")
	return a.articlePage(r, false)
}

// adminArticles 仅允许平台管理员搜索全部文章状态。
func (a *App) adminArticles(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	return a.articlePage(r, true)
}

// publicArticle 只输出当前已发布文字的安全HTML，审核修改原稿后不会沿用旧渲染缓存。
func (a *App) publicArticle(w http.ResponseWriter, r *http.Request) (any, error) {
	v, err := get[Article](r.Context(), a.DB, "articles", r.PathValue("id"), false)
	if missing(err) || err == nil && v.Status != "published" {
		return nil, fail(404, "文章不存在或已下架")
	}
	if err != nil {
		return nil, err
	}
	body, _, err := renderArticleMarkdown(r.Context(), a.DB, v.BodyMarkdown, "", User{}, false)
	if err != nil {
		return nil, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return struct {
		ArticleSummary
		BodyHTML string `json:"body_html"`
	}{v.ArticleSummary, body}, nil
}

// adminArticle 返回管理员可编辑的Markdown原稿，普通用户不能访问。
func (a *App) adminArticle(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	v, err := get[Article](r.Context(), a.DB, "articles", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	return adminArticleView(v), nil
}

// saveArticle 在同一事务保存版本、正文、托管图引用、后置审核和管理员审计。
func (a *App) saveArticle(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Title        string `json:"title"`
		Summary      string `json:"summary"`
		BodyMarkdown string `json:"body_markdown"`
		CoverURL     string `json:"cover_url"`
		Status       string `json:"status"`
		Revision     *int64 `json:"revision"`
	}
	if err = decodeWithLimit(r, &in, articleJSONTransportMaxBytes); err != nil {
		return nil, err
	}
	if !length(in.Title, 2, 60) || !length(in.Summary, 0, 300) || !contains([]string{"draft", "published", "hidden"}, in.Status) {
		return nil, fail(400, "文章标题2至60字、摘要最多300字，请选择有效状态")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	u, err = lockManagementWriteActor(r.Context(), tx, u.ID)
	if err != nil {
		return nil, err
	}
	if u.Role != "admin" {
		return nil, fail(403, "文章管理权限已失效")
	}
	v := Article{ArticleSummary: ArticleSummary{ID: newID(), CreatedAt: now()}}
	if id := r.PathValue("id"); id != "" {
		v, err = get[Article](r.Context(), tx, "articles", id, true)
		if err != nil {
			return nil, err
		}
		if in.Revision == nil || *in.Revision != v.Revision {
			return nil, fail(409, "文章已更新，请刷新后保存")
		}
	}
	if err = validateContentImage(r.Context(), tx, in.CoverURL, v.CoverURL, "article_cover", u); err != nil {
		return nil, err
	}
	_, images, err := renderArticleMarkdown(r.Context(), tx, in.BodyMarkdown, v.BodyMarkdown, u, true)
	if err != nil {
		return nil, err
	}
	v.Title, v.Summary, v.BodyMarkdown, v.CoverURL, v.Status = strings.TrimSpace(in.Title), in.Summary, in.BodyMarkdown, in.CoverURL, in.Status
	v.BodyImages, v.EditorID = images, u.ID
	v.UpdatedAt = now()
	v.Revision++
	if v.Status == "published" && v.PublishedAt == "" {
		v.PublishedAt = v.UpdatedAt
	}
	if err = put(r.Context(), tx, "articles", v.ID, v); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "article.save", v.ID, "保存或调整Markdown文章发布状态"); err != nil {
		return nil, err
	}
	return adminArticleView(v), tx.Commit(r.Context())
}

// previewArticle 使用与公开详情相同的安全解析器预览，但不保存草稿或创建审核任务。
func (a *App) previewArticle(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		BodyMarkdown string `json:"body_markdown"`
	}
	if err = decodeWithLimit(r, &in, articleJSONTransportMaxBytes); err != nil {
		return nil, err
	}
	body, _, err := renderArticleMarkdown(r.Context(), a.DB, in.BodyMarkdown, "", u, false)
	return map[string]string{"body_html": body}, err
}
