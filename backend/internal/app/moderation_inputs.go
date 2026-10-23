// Package app 在持锁之前按领域权限与尺寸约束选择需要审核的用户内容。
package app

import (
	"net/http"
	"strings"
)

// moderateProfileText 校验本人昵称简介后审核，未修改字段不发送到外部模型。
func (a *App) moderateProfileText(r *http.Request, u User, nickname, bio *string) error {
	parts := []string{}
	if nickname != nil {
		if !length(*nickname, 2, 24) {
			return fail(400, "昵称需为 2 至 24 个字")
		}
		parts = append(parts, *nickname)
	}
	if bio != nil {
		if !length(*bio, 0, 300) {
			return fail(400, "简介最多 300 字")
		}
		parts = append(parts, *bio)
	}
	return a.moderateText(r, u, "个人资料", parts...)
}

// moderateServiceDraft 先检查编辑归属及完整服务边界，审核不占用服务或资金行锁。
func (a *App) moderateServiceDraft(r *http.Request, u User, s Service) error {
	if id := r.PathValue("id"); id != "" {
		previous, err := get[Service](r.Context(), a.DB, "services", id, false)
		if err != nil {
			return err
		}
		if previous.ProviderID != u.ID && u.Role != "admin" {
			return fail(403, "只能编辑自己的服务")
		}
	}
	game, err := get[Game](r.Context(), a.DB, "games", s.GameID, false)
	if err != nil || !game.Enabled {
		return fail(400, "游戏当前不可用")
	}
	if err = validateService(&s, game); err != nil {
		return err
	}
	parts := []string{s.Title, s.Description, s.Category, s.Unit}
	parts = append(parts, s.Tags...)
	for _, label := range s.Labels {
		parts = append(parts, label.Text)
	}
	return a.moderateText(r, u, "服务发布", parts...)
}

// moderateClubDraft 校验组织编辑权限与新传文字尺寸，图片在不可变上传入口单独审核。
func (a *App) moderateClubDraft(r *http.Request, u User, name, description, assessment *string, blocks *[]CommunityBlock) error {
	if id := r.PathValue("id"); id != "" {
		if _, err := a.clubManager(r, u, id); err != nil {
			return err
		}
	}
	parts := []string{}
	for _, f := range []struct {
		value    *string
		min, max int
		label    string
	}{{name, 2, 30, "俱乐部名称需为2至30字"}, {description, 10, 1000, "俱乐部介绍需为10至1000字"}, {assessment, 0, 1000, "考核说明最多1000字"}} {
		if f.value != nil {
			if !length(*f.value, f.min, f.max) {
				return fail(400, f.label)
			}
			parts = append(parts, *f.value)
		}
	}
	if blocks != nil {
		if len(*blocks) > 20 {
			return fail(400, "详情最多20段")
		}
		total := 0
		for _, block := range *blocks {
			if block.Type == "text" {
				if !length(block.Text, 1, 2000) {
					return fail(400, "详情文字每段1至2000字")
				}
				total += len([]rune(block.Text))
				parts = append(parts, block.Text)
			}
		}
		if total > 8000 {
			return fail(400, "详情最多8000字")
		}
	}
	return a.moderateText(r, u, "俱乐部资料", parts...)
}

// moderateOrderNote 在事务前验证动作权限和说明，事务内仍重新检查实时订单状态。
func (a *App) moderateOrderNote(r *http.Request, u User, action, note string) error {
	if !length(note, 0, 2000) {
		return fail(400, "说明最多 2000 字")
	}
	if strings.TrimSpace(note) == "" {
		return nil
	}
	o, err := get[Order](r.Context(), a.DB, "orders", r.PathValue("id"), false)
	if err != nil {
		return err
	}
	if err = applyAction(&o, u, action, note); err != nil {
		return err
	}
	return a.moderateText(r, u, "订单说明", note)
}

// imageModerationStamp 将所有新上传标为待审，开发环境也不伪造正式通过证明。
func (a *App) imageModerationStamp() string {
	return "pending"
}

// moderateStoredImage 兼容历史调用，仅确认图片存在；新引用由业务写入钩子统一入队。
func (a *App) moderateStoredImage(r *http.Request, u User, id, scope string) error {
	_, err := get[Evidence](r.Context(), a.DB, "evidence", id, false)
	return err
}

// moderateOwnedImage 先校验图片用途与归属，避免未经授权的图片被发送给外部审核服务。
func (a *App) moderateOwnedImage(r *http.Request, u User, raw, purpose, ownerID, clubID string) error {
	if raw == "" || raw == rejectedImageURL {
		return nil
	}
	if trustedDefaultAvatar(raw, purpose) {
		return nil
	}
	if err := validateOwnedImage(r.Context(), a.DB, raw, purpose, ownerID, clubID, u); err != nil {
		return err
	}
	match := publicImagePath.FindStringSubmatch(raw)
	if match == nil {
		return fail(400, "请使用本平台上传的图片")
	}
	return a.moderateStoredImage(r, u, match[1], "引用历史公开图片："+purpose)
}
