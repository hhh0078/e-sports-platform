// Package app 仅搜索及解析真实公开服务、需求和文章，目标DTO不包含订单参与者和支付秘密。
package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ContentTargetOption 是后台搜索使用的最小公开目标摘要。
type ContentTargetOption struct {
	ContentTarget
	Title    string `json:"title"`
	ImageURL string `json:"image_url"`
	GameID   string `json:"game_id"`
	Subtitle string `json:"subtitle"`
}

// contentTargetWhere 构造固定白名单的公开可见条件，全部用户关键词使用绑定参数。
func contentTargetWhere(kind string) (string, string, string, error) {
	switch kind {
	case "service":
		return "services", "title", "left(id,5)<>'demo_' AND coalesce(data->>'is_demo','false')<>'true' AND left(coalesce(data->>'provider_id',''),5)<>'demo_' AND left(coalesce(data->>'club_id',''),5)<>'demo_' AND data->>'status'='active' AND EXISTS(SELECT 1 FROM users u WHERE u.id=services.data->>'provider_id' AND u.data->>'status'='active') AND EXISTS(SELECT 1 FROM games g WHERE g.id=services.data->>'game_id' AND g.data->>'enabled'='true')", nil
	case "request":
		return "orders", "title", "left(id,5)<>'demo_' AND coalesce(data->>'is_demo','false')<>'true' AND left(coalesce(data->>'buyer_id',''),5)<>'demo_' AND coalesce(data->>'service_id','')='' AND data->>'status'='open' AND coalesce(data->>'provider_id','')='' AND data->>'payment_status'='paid' AND coalesce(data->>'refund_status','none') NOT IN ('requested','escalated','processing') AND EXISTS(SELECT 1 FROM users u WHERE u.id=orders.data->>'buyer_id' AND u.data->>'status'='active') AND EXISTS(SELECT 1 FROM games g WHERE g.id=orders.data->>'game_id' AND g.data->>'enabled'='true')", nil
	case "article":
		return "articles", "title", "data->>'status'='published'", nil
	}
	return "", "", "", fail(400, "跳转仅支持服务、需求和文章")
}

// validateContentTarget 在保存时核对目标当前可公开，禁止把下架或私密记录写成跳转。
func (a *App) validateContentTarget(ctx context.Context, q querier, target *ContentTarget) error {
	if !validContentTarget(target) {
		return fail(400, "跳转类型或编号不正确")
	}
	if target == nil {
		return nil
	}
	table, _, where, err := contentTargetWhere(target.Kind)
	if err != nil {
		return err
	}
	var exists bool
	if err = q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE id=$1 AND "+where+")", target.ID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fail(404, "跳转目标不存在或已下架")
	}
	return nil
}

// liveContentTarget 每次公开读取时重新校验目标，失效时撤销行动而保留运营图片。
func (a *App) liveContentTarget(ctx context.Context, target *ContentTarget) *ContentTarget {
	if target != nil && a.validateContentTarget(ctx, a.DB, target) == nil {
		copy := *target
		return &copy
	}
	return nil
}

// searchContentTargets 在完整公开集合中搜索再分页，不复用可见私密记录的管理大全列表。
func (a *App) searchContentTargets(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	kind := r.URL.Query().Get("kind")
	table, title, where, err := contentTargetWhere(kind)
	if err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if !length(keyword, 0, 100) {
		return nil, fail(400, "搜索关键词最多100字")
	}
	limit, offset := pageLimit(r)
	where += " AND ($1='' OR strpos(lower(concat_ws(' ',id,data->>'" + title + "')),lower($1))>0)"
	var total int
	if err = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM "+table+" WHERE "+where, keyword).Scan(&total); err != nil {
		return nil, err
	}
	items, err := list[map[string]any](r.Context(), a.DB, table, fmt.Sprintf("WHERE %s ORDER BY data->>'created_at' DESC,id DESC LIMIT $2 OFFSET $3", where), keyword, limit, offset)
	if err != nil {
		return nil, err
	}
	out := []ContentTargetOption{}
	for _, item := range items {
		game := reviewString(item, "game_id")
		image := reviewString(item, "cover_url")
		if kind == "request" {
			if g, e := get[Game](r.Context(), a.DB, "games", game, false); e == nil {
				image = g.CoverURL
			}
		}
		out = append(out, ContentTargetOption{ContentTarget: ContentTarget{Kind: kind, ID: reviewString(item, "id")}, Title: reviewString(item, "title"), ImageURL: image, GameID: game, Subtitle: reviewString(item, "category")})
	}
	return map[string]any{"items": out, "total": total}, nil
}
