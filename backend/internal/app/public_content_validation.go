// Package app 校验运营图文的用途、归属、游戏分组与长度，所有规则在服务端执行。
package app

import (
	"context"
	"errors"
	"strings"

	"jijie/internal/assets"
)

// validateContentTargetEdit 仅撤销同一字段未修改且已失效的旧跳转，新增对象或跨游戏复制仍严格核验公开资格。
func (a *App) validateContentTargetEdit(ctx context.Context, q querier, target **ContentTarget, previous *ContentTarget) error {
	err := a.validateContentTarget(ctx, q, *target)
	var unavailable *apiError
	if err != nil && errors.As(err, &unavailable) && unavailable.Status == 404 && *target != nil && previous != nil && **target == *previous {
		*target = nil
		return nil
	}
	return err
}

// validateContentImage 允许可信内置素材或本用途的公开托管图片，旧引用不因管理员换人丢失。
func validateContentImage(ctx context.Context, q querier, raw, previous, purpose string, actor User) error {
	if raw == "" || raw == rejectedImageURL {
		return nil
	}
	if strings.HasPrefix(raw, "/static/operations/") {
		if _, err := assets.OperationImageData(raw, purpose); err != nil {
			return fail(400, "平台内置图片不存在或用途不匹配")
		}
		return nil
	}
	if localCover.MatchString(raw) {
		if _, err := assets.PublicCoverData(raw); err != nil {
			return fail(400, "内置图片不存在")
		}
		return nil
	}
	match := publicImagePath.FindStringSubmatch(raw)
	if match == nil {
		return fail(400, "请使用平台托管图片，不支持外部图片或订单证据")
	}
	image, err := get[PublicImage](ctx, q, "evidence", match[1], false)
	if err != nil || !image.Public || image.OrderID != "" || image.Purpose != purpose || image.Moderation == "rejected" || len(image.Data) == 0 || image.ClubID != "" {
		return fail(400, "公共图片不存在、被撤销或用途不匹配")
	}
	if raw != previous && image.OwnerID != actor.ID {
		return fail(403, "新增图片必须由当前管理员上传")
	}
	return validateImageDimensions(purpose, image.Width, image.Height)
}

// validateContentSlides 校验稳定图层ID、最多一个游戏槽及各图片独立文字，排序由数组决定。
func (a *App) validateContentSlides(ctx context.Context, q querier, slides []ContentSlide, previous []ContentSlide, gameList bool, actor User) ([]ContentSlide, error) {
	if len(slides) > 12 {
		return nil, fail(400, "每组轮播最多12项")
	}
	old := map[string]ContentSlide{}
	for _, slide := range previous {
		old[slide.ID] = slide
	}
	out, seen, slots := []ContentSlide{}, map[string]bool{}, 0
	for _, slide := range slides {
		if slide.ID == "" {
			slide.ID = newID()
		}
		if !length(slide.ID, 1, 80) || seen[slide.ID] {
			return nil, fail(400, "轮播标识重复或无效")
		}
		seen[slide.ID] = true
		switch slide.Kind {
		case "game_slot":
			slots++
			if gameList || slots > 1 || slide.ImageURL != "" || len(slide.Labels) != 0 || slide.Target != nil {
				return nil, fail(400, "全局仅支持一个游戏槽，游戏槽不能包含图片文字或跳转")
			}
			slide.Labels = []CoverLabel{}
		case "image":
			if slide.ImageURL == "" {
				return nil, fail(400, "请上传轮播图片")
			}
			if err := validateContentImage(ctx, q, slide.ImageURL, old[slide.ID].ImageURL, "promotion_banner", actor); err != nil {
				return nil, err
			}
			labels, err := normalizeContentLabels(slide.Labels)
			if err != nil {
				return nil, err
			}
			slide.Labels = labels
			if err = a.validateContentTargetEdit(ctx, q, &slide.Target, old[slide.ID].Target); err != nil {
				return nil, err
			}
		default:
			return nil, fail(400, "轮播类型不支持")
		}
		out = append(out, slide)
	}
	return out, nil
}

// validatePublicContent 在保存前统一核对全部配置，注意事项保留原换行而非合并成单行。
func (a *App) validatePublicContent(ctx context.Context, q querier, v *PublicContentSettings, previous PublicContentSettings, actor User) error {
	previousTips := []ContentPageTip{previous.PageTips.Player, previous.PageTips.Worker, previous.PageTips.Orders, previous.PageTips.Me}
	for index, tip := range []*ContentPageTip{&v.PageTips.Player, &v.PageTips.Worker, &v.PageTips.Orders, &v.PageTips.Me} {
		if !length(tip.Text, 0, 300) {
			return fail(400, "页面顶部提示最多300字")
		}
		if tip.ArticleID != "" {
			target := &ContentTarget{Kind: "article", ID: tip.ArticleID}
			var oldTarget *ContentTarget
			if previousTips[index].ArticleID != "" {
				oldTarget = &ContentTarget{Kind: "article", ID: previousTips[index].ArticleID}
			}
			if err := a.validateContentTargetEdit(ctx, q, &target, oldTarget); err != nil {
				return err
			}
			if target == nil {
				tip.ArticleID = ""
			}
		}
	}
	if !length(v.ServiceNotice, 0, 2000) || !length(v.RequestNotice, 0, 2000) {
		return fail(400, "服务和需求注意事项各最多2000字")
	}
	notice := &v.Announcement
	if !contains([]string{"always", "once"}, notice.Frequency) || !contains([]string{"image", "text"}, notice.Mode) || !length(notice.Title, 0, 60) || !length(notice.Body, 0, 3000) {
		return fail(400, "公告模式、频率或文字长度不正确")
	}
	if notice.Enabled && (notice.Mode == "image" && notice.ImageURL == "" || notice.Mode == "text" && strings.TrimSpace(notice.Body) == "") {
		return fail(400, "启用公告前请填写对应文字或图片")
	}
	if err := validateContentImage(ctx, q, notice.ImageURL, previous.Announcement.ImageURL, "announcement_image", actor); err != nil {
		return err
	}
	if err := a.validateContentTargetEdit(ctx, q, &notice.Target, previous.Announcement.Target); err != nil {
		return err
	}
	var err error
	v.GlobalSlides, err = a.validateContentSlides(ctx, q, v.GlobalSlides, previous.GlobalSlides, false, actor)
	if err != nil {
		return err
	}
	if len(v.GameSlides) > 100 {
		return fail(400, "游戏轮播分组过多")
	}
	for gameID, slides := range v.GameSlides {
		if !gameIDPattern.MatchString(gameID) {
			return fail(400, "游戏编号不正确")
		}
		if _, err = get[Game](ctx, q, "games", gameID, false); err != nil {
			return fail(400, "轮播关联游戏不存在")
		}
		v.GameSlides[gameID], err = a.validateContentSlides(ctx, q, slides, previous.GameSlides[gameID], true, actor)
		if err != nil {
			return err
		}
	}
	return nil
}
