// Package app 提供公开玩家名片、个人资料编辑和俱乐部结构化装修校验。
package app

import (
	"context"
	"net/http"
	"strings"
)

// CommunityBlock 只允许纯文本或已托管图片，客户端无需渲染用户提供的 HTML。
type CommunityBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// saveProfile 在行锁内更新本人公开资料，不接受角色、手机号或认证等权限字段。
func (a *App) saveProfile(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Nickname     *string `json:"nickname"`
		Bio          *string `json:"bio"`
		Avatar       *string `json:"avatar"`
		ProfileCover *string `json:"profile_cover"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if err = a.moderateProfileText(r, actor, in.Nickname, in.Bio); err != nil {
		return nil, err
	}
	for _, imageField := range []struct {
		value   *string
		purpose string
	}{{in.Avatar, "avatar"}, {in.ProfileCover, "profile_cover"}} {
		if imageField.value != nil {
			if err = a.moderateOwnedImage(r, actor, *imageField.value, imageField.purpose, actor.ID, ""); err != nil {
				return nil, err
			}
		}
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	u, err := get[User](r.Context(), tx, "users", actor.ID, true)
	if err != nil {
		return nil, err
	}
	if in.Nickname != nil {
		if !length(*in.Nickname, 2, 24) {
			return nil, fail(400, "昵称需为 2 至 24 个字")
		}
		u.Nickname = strings.TrimSpace(*in.Nickname)
	}
	if in.Bio != nil {
		if !length(*in.Bio, 0, 300) {
			return nil, fail(400, "简介最多 300 字")
		}
		u.Bio = strings.TrimSpace(*in.Bio)
	}
	if in.Avatar != nil {
		if err = validateOwnedImage(r.Context(), tx, *in.Avatar, "avatar", u.ID, "", actor); err != nil {
			return nil, err
		}
		u.Avatar = *in.Avatar
	}
	if in.ProfileCover != nil {
		if err = validateOwnedImage(r.Context(), tx, *in.ProfileCover, "profile_cover", u.ID, "", actor); err != nil {
			return nil, err
		}
		u.ProfileCover = *in.ProfileCover
	}
	if err = put(r.Context(), tx, "users", u.ID, u); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "profile.edit", u.ID, ""); err != nil {
		return nil, err
	}
	return u, tx.Commit(r.Context())
}

// playerProfile 手工选择公开字段，隐藏停用账号、未发布服务和申请中的俱乐部关系。
func (a *App) playerProfile(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := get[User](r.Context(), a.DB, "users", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		return nil, fail(404, "玩家不存在")
	}
	profile := map[string]any{"id": u.ID, "nickname": u.Nickname, "avatar": u.Avatar, "bio": u.Bio, "profile_cover": u.ProfileCover, "provider_enabled": u.ProviderEnabled}
	services, err := a.publicCommunityServices(r, "provider", u.ID)
	if err != nil {
		return nil, err
	}
	clubs, err := list[Club](r.Context(), a.DB, "clubs", "WHERE data->>'status'='active' AND (data->>'owner_id'=$1 OR EXISTS (SELECT 1 FROM members m WHERE m.data->>'club_id'=clubs.id AND m.data->>'user_id'=$1 AND m.data->>'status'='active')) ORDER BY data->>'created_at' DESC LIMIT 100", u.ID)
	if err != nil {
		return nil, err
	}
	for i := range clubs {
		clubs[i] = a.decorateClub(r.Context(), clubs[i])
	}
	return map[string]any{"profile": profile, "services": services, "clubs": clubs}, nil
}

// publicCommunityServices 查询当前可售服务，俱乐部主页额外校验打手仍属于该组织。
func (a *App) publicCommunityServices(r *http.Request, scope, id string) ([]Service, error) {
	where := "WHERE data->>'status'='active' AND EXISTS(SELECT 1 FROM games g WHERE g.id=services.data->>'game_id' AND g.data->>'enabled'='true') AND EXISTS(SELECT 1 FROM users u WHERE u.id=services.data->>'provider_id' AND u.data->>'status'='active')"
	if scope == "club" {
		where += " AND data->>'club_id'=$1 AND EXISTS(SELECT 1 FROM clubs c WHERE c.id=$1 AND c.data->>'status'='active' AND c.data->'game_ids' ? (services.data->>'game_id') AND (c.data->>'owner_id'=services.data->>'provider_id' OR EXISTS(SELECT 1 FROM members m WHERE m.data->>'club_id'=c.id AND m.data->>'user_id'=services.data->>'provider_id' AND m.data->>'status'='active')))"
	} else {
		where += " AND data->>'provider_id'=$1"
	}
	limit, offset := pageLimit(r)
	items, err := list[Service](r.Context(), a.DB, "services", where+" ORDER BY data->>'created_at' DESC, id DESC LIMIT $2 OFFSET $3", id, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i] = a.decorateService(r.Context(), items[i])
	}
	return items, nil
}

// validateClubDecoration 检查图文块、门槛及图片归属，押金金额仅表示俱乐部公布的规则。
func validateClubDecoration(ctx context.Context, q querier, c *Club, actor User) error {
	if c.DepositCents < 0 || c.DepositCents > 10000000 || !length(c.Assessment, 0, 1000) || len(c.DetailBlocks) > 20 {
		return fail(400, "押金门槛需为 0 至 100000 元，考核说明最多 1000 字，详情最多 20 段")
	}
	c.Assessment = strings.TrimSpace(c.Assessment)
	if err := validateOwnedImage(ctx, q, c.CoverURL, "club_cover", c.OwnerID, c.ID, actor); err != nil {
		return err
	}
	imageCount, textCount := 0, 0
	for i := range c.DetailBlocks {
		block := &c.DetailBlocks[i]
		switch block.Type {
		case "text":
			if !length(block.Text, 1, 2000) || block.ImageURL != "" {
				return fail(400, "文字段需为 1 至 2000 字，不能同时设置图片")
			}
			block.Text = strings.TrimSpace(block.Text)
			textCount += len([]rune(block.Text))
		case "image":
			if block.ImageURL == "" || block.Text != "" {
				return fail(400, "图片段只允许设置托管图片")
			}
			if err := validateOwnedImage(ctx, q, block.ImageURL, "club_detail", c.OwnerID, c.ID, actor); err != nil {
				return err
			}
			imageCount++
		default:
			return fail(400, "详情只支持文字段或图片段")
		}
	}
	if imageCount > 10 || textCount > 8000 {
		return fail(400, "详情最多 10 张图片、8000 字")
	}
	return nil
}

// communityMemberView 仅向本人和管理者展示申请资料，公开认证不代表押金托管。
func communityMemberView(m ClubMember, u User, private bool) map[string]any {
	out := map[string]any{"user_id": m.UserID, "nickname": u.Nickname, "avatar": u.Avatar, "club_id": m.ClubID, "status": m.Status, "certified": m.Status == "active" && u.Status == "active"}
	if private {
		out["created_at"] = m.CreatedAt
		out["application_note"] = m.ApplicationNote
		out["requirements_accepted"] = m.RequirementsAccepted
		out["applied_deposit_cents"] = m.AppliedDepositCents
		out["applied_assessment"] = m.AppliedAssessment
		out["assessment_passed"] = m.AssessmentPassed
		out["review_note"] = m.ReviewNote
		out["deposit_status"] = "not_required"
		if m.AppliedDepositCents > 0 {
			out["deposit_status"] = "unverified"
		}
	}
	return out
}
