// Package app 实现多俱乐部隔离、挂靠审核、组织认证及历史订单权限。
package app

import (
	"context"
	"net/http"
	"strings"
)

// decorateClub 计算当前有效成员数量与负责人昵称。
func (a *App) decorateClub(ctx context.Context, c Club) Club {
	if c.AvatarURL == "" {
		c.AvatarURL = defaultClubAvatar(c.ID)
	}
	settings, settingsErr := financialSettings(ctx, a.DB)
	if settingsErr == nil {
		c.PlatformDepositRequiredCents = clubRequiredDeposit(c, settings)
	}
	c.PlatformDepositCents = 0
	c.DepositReady = false
	if wallet, e := accountBalance(ctx, a.DB, clubDepositAccount(c.ID)); e == nil {
		c.PlatformDepositRefundingCents = wallet.DepositRefundingCents
		c.PlatformDepositShortfallCents = depositShortfall(wallet.DepositCents, c.PlatformDepositRequiredCents)
		if settingsErr == nil {
			c.PlatformDepositCents = qualifiedPlatformDeposit(wallet, c.PlatformDepositRequiredCents)
			c.DepositReady = wallet.DepositRefundingCents == 0 && wallet.DepositCents >= c.PlatformDepositRequiredCents
		}
	}
	if u, e := get[User](ctx, a.DB, "users", c.OwnerID, false); e == nil {
		c.OwnerName = u.Nickname
	}
	var n int
	if a.DB.QueryRow(ctx, "SELECT count(*) FROM members WHERE data->>'club_id'=$1 AND data->>'status'='active' AND EXISTS(SELECT 1 FROM users u WHERE u.id=members.data->>'user_id' AND u.data->>'status'='active')", c.ID).Scan(&n) == nil {
		c.MemberCount = n
	}
	return c
}

// clubs 返回公开俱乐部或当前用户创建及申请挂靠的俱乐部。
func (a *App) clubs(w http.ResponseWriter, r *http.Request) (any, error) {
	if r.URL.Query().Get("managed") == "1" {
		return a.managedClubs(w, r)
	}
	mine := r.URL.Query().Get("mine") == "1"
	var u User
	var err error
	if mine {
		u, err = a.actor(r)
		if err != nil {
			return nil, err
		}
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !length(query, 0, 100) {
		return nil, fail(400, "搜索关键词最多 100 字")
	}
	where, args := "ORDER BY data->>'created_at' DESC, id DESC", []any{}
	if query != "" {
		// 搜索词作为绑定参数按普通文本匹配，先过滤再应用挂靠范围和分页。
		where = "WHERE strpos(lower(concat_ws(' ',data->>'name',data->>'description')),lower($1))>0 " + where
		args = append(args, query)
	}
	items, err := list[Club](r.Context(), a.DB, "clubs", where, args...)
	if err != nil {
		return nil, err
	}
	out := []Club{}
	for _, c := range items {
		if mine {
			m, e := get[ClubMember](r.Context(), a.DB, "members", c.ID+":"+u.ID, false)
			if c.OwnerID != u.ID && e != nil {
				continue
			}
			if c.OwnerID == u.ID {
				c.MembershipStatus = "owner"
			} else {
				c.MembershipStatus = m.Status
			}
		} else if c.Status != "active" {
			continue
		}
		out = append(out, a.decorateClub(r.Context(), c))
	}
	return paginate(r, out), nil
}

// club 返回成员视图，申请资料只向本人、组织负责人及管理员展示。
func (a *App) club(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := get[Club](r.Context(), a.DB, "clubs", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	u, _ := a.actor(r)
	manager, checkErr := canManageClub(r.Context(), a.DB, u, c)
	if checkErr != nil {
		return nil, checkErr
	}
	managementRole, checkErr := clubManagementRole(r.Context(), a.DB, u, c)
	if checkErr != nil {
		return nil, checkErr
	}
	if c.Status != "active" && !manager {
		return nil, fail(404, "俱乐部尚未通过审核")
	}
	members, err := list[ClubMember](r.Context(), a.DB, "members", "WHERE data->>'club_id'=$1 ORDER BY data->>'created_at'", c.ID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	var my map[string]any
	for _, m := range members {
		member, e := get[User](r.Context(), a.DB, "users", m.UserID, false)
		if e != nil {
			continue
		}
		m.Nickname = member.Nickname
		if m.UserID == u.ID {
			my = a.memberFinancialView(r.Context(), m, member, true)
		}
		if manager || m.Status == "active" && member.Status == "active" {
			out = append(out, a.memberFinancialView(r.Context(), m, member, manager))
		}
	}
	services, err := a.publicCommunityServices(r, "club", c.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"club": a.decorateClub(r.Context(), c), "members": out, "my_membership": my, "services": services, "can_manage": manager, "management_role": managementRole, "can_manage_admins": canManageClubAdmins(u, c)}, nil
}

// validateClub 校验组织基本资料与游戏范围。
func validateClub(ctx context.Context, q querier, c Club) error {
	if !length(c.Name, 2, 30) || !length(c.Description, 10, 1000) || len(c.GameIDs) < 1 || len(c.GameIDs) > 32 {
		return fail(400, "俱乐部名称 2 至 30 字、介绍 10 至 1000 字，选择 1 至 32 个游戏")
	}
	seen := map[string]bool{}
	for _, id := range c.GameIDs {
		g, err := get[Game](ctx, q, "games", id, false)
		if err != nil || !g.Enabled || seen[id] {
			return fail(400, "游戏不存在、未启用或重复")
		}
		seen[id] = true
	}
	return nil
}

// saveClub 创建组织走押金及认证流程；既有组织的资料编辑只加入独立后置审核。
func (a *App) saveClub(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if r.PathValue("id") == "" {
		if err = a.requireVerifiedPhone(r, u); err != nil {
			return nil, err
		}
	}
	var in struct {
		ContactQRURL      *string           `json:"contact_qr_url"`
		ContactPhone      *string           `json:"contact_phone"`
		ContactWechat     *string           `json:"contact_wechat"`
		RequestID         string            `json:"request_id"`
		Name              *string           `json:"name"`
		Description       *string           `json:"description"`
		GameIDs           *[]string         `json:"game_ids"`
		CoverURL          *string           `json:"cover_url"`
		DetailBlocks      *[]CommunityBlock `json:"detail_blocks"`
		DepositCents      *int64            `json:"deposit_cents"`
		Assessment        *string           `json:"assessment"`
		CommissionRateBPS *int64            `json:"commission_rate_bps"`
		JoinNote          *string           `json:"join_note"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	normalizeContactPatch(in.ContactPhone, in.ContactWechat, in.ContactQRURL)
	if in.JoinNote != nil {
		if err = a.moderateText(r, u, "俱乐部加入备注", *in.JoinNote); err != nil {
			return nil, err
		}
	}
	if err = a.moderateClubDraft(r, u, in.Name, in.Description, in.Assessment, in.DetailBlocks); err != nil {
		return nil, err
	}
	if in.ContactPhone != nil || in.ContactWechat != nil {
		phone, wechat := "", ""
		if in.ContactPhone != nil {
			phone = *in.ContactPhone
		}
		if in.ContactWechat != nil {
			wechat = *in.ContactWechat
		}
		if err = validateSupportContact(phone, wechat); err != nil {
			return nil, err
		}
		if err = a.moderateContactPatch(r, u, "俱乐部客服联系方式", in.ContactPhone, in.ContactWechat); err != nil {
			return nil, err
		}
	}
	ownerID, clubID := u.ID, r.PathValue("id")
	if clubID != "" {
		old, e := a.clubManager(r, u, clubID)
		if e != nil {
			return nil, e
		}
		ownerID = old.OwnerID
	}
	if in.ContactQRURL != nil {
		previous := ""
		if clubID != "" {
			old, e := a.clubManager(r, u, clubID)
			if e != nil {
				return nil, e
			}
			previous = old.ContactQRURL
		}
		if *in.ContactQRURL != previous {
			if err = a.moderateOwnedImage(r, u, *in.ContactQRURL, "club_contact_qr", u.ID, clubID); err != nil {
				return nil, err
			}
		}
	}
	if in.CoverURL != nil {
		if err = a.moderateOwnedImage(r, u, *in.CoverURL, "club_cover", ownerID, clubID); err != nil {
			return nil, err
		}
	}
	if in.DetailBlocks != nil {
		for _, block := range *in.DetailBlocks {
			if block.Type == "image" {
				if err = a.moderateOwnedImage(r, u, block.ImageURL, "club_detail", ownerID, clubID); err != nil {
					return nil, err
				}
			}
		}
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	c := Club{ID: newID(), OwnerID: u.ID, OwnerName: u.Nickname, CreatedAt: now()}
	if r.PathValue("id") == "" && in.RequestID != "" {
		if err = walletRequestID(in.RequestID); err != nil {
			return nil, err
		}
		if err = lockWalletUsers(r.Context(), tx, u.ID); err != nil {
			return nil, err
		}
		if err = claimWalletRequest(r.Context(), tx, u.ID, in.RequestID, "club_create", in); err != nil {
			return nil, err
		}
		c.ID = walletKey("club-create", u.ID, in.RequestID)
		old, e := get[Club](r.Context(), tx, "clubs", c.ID, false)
		if e == nil {
			return old, tx.Commit(r.Context())
		} else if !missing(e) {
			return nil, e
		}
	}

	if id := r.PathValue("id"); id != "" {
		if err = lockClubFinance(r.Context(), tx, id); err != nil {
			return nil, err
		}
		u, err = lockManagementWriteActor(r.Context(), tx, u.ID)
		if err != nil {
			return nil, err
		}
		c, err = get[Club](r.Context(), tx, "clubs", id, true)
		if err != nil {
			return nil, err
		}
		if allowed, permissionErr := canManageClub(r.Context(), tx, u, c); permissionErr != nil {
			return nil, permissionErr
		} else if !allowed {
			return nil, fail(403, "只能管理自己的俱乐部")
		}
	}
	if in.ContactQRURL != nil {
		if *in.ContactQRURL != c.ContactQRURL {
			if err = validateOwnedImage(r.Context(), tx, *in.ContactQRURL, "club_contact_qr", u.ID, c.ID, u); err != nil {
				return nil, err
			}
		}
		c.ContactQRURL = *in.ContactQRURL
	}
	if in.ContactPhone != nil {
		c.ContactPhone = *in.ContactPhone
	}
	if in.ContactWechat != nil {
		c.ContactWechat = *in.ContactWechat
	}
	if err = validateSupportContact(c.ContactPhone, c.ContactWechat); err != nil {
		return nil, err
	}
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		c.Description = strings.TrimSpace(*in.Description)
	}
	if in.GameIDs != nil {
		c.GameIDs = *in.GameIDs
	}
	if in.CoverURL != nil {
		c.CoverURL = *in.CoverURL
	}
	if in.DetailBlocks != nil {
		c.DetailBlocks = *in.DetailBlocks
	}
	if in.DepositCents != nil {
		c.DepositCents = *in.DepositCents
	}
	if in.Assessment != nil {
		c.Assessment = *in.Assessment
	}
	if in.CommissionRateBPS != nil {
		c.CommissionRateBPS = *in.CommissionRateBPS
	}
	if in.JoinNote != nil {
		c.JoinNote = strings.TrimSpace(*in.JoinNote)
	}
	if c.CommissionRateBPS < 0 || c.CommissionRateBPS > 10000 || !length(c.JoinNote, 0, 2000) {
		return nil, fail(400, "俱乐部抽成须在0至100%，加入备注最多2000字")
	}
	if err = lockFinancialSettings(r.Context(), tx); err != nil {
		return nil, err
	}
	settings, err := financialSettings(r.Context(), tx)
	if err != nil {
		return nil, err
	}
	if c.CommissionRateBPS+settings.ClubPlatformRateBPS > 10000 {
		return nil, fail(400, "俱乐部与平台抽成合计不能超过100%")
	}
	if c.AvatarURL == "" {
		c.AvatarURL = defaultClubAvatar(c.ID)
	}
	if r.PathValue("id") == "" {
		c.CreationDepositVersion = 1
		c.CreationDepositRequiredCents = settings.ClubCreationDepositCents
	}
	// 图文编辑与组织认证独立；仅创建时进入押金及组织审核流程。
	if r.PathValue("id") == "" {
		c.Status = "pending"
		c.Verified = false
		if settings.ClubCreationDepositCents > 0 {
			c.Status = "pending_deposit"
		}
	}
	if err = validateClub(r.Context(), tx, c); err != nil {
		return nil, err
	}
	if err = validateClubDecoration(r.Context(), tx, &c, u); err != nil {
		return nil, err
	}
	if err = put(r.Context(), tx, "clubs", c.ID, c); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "club.edit", c.ID, ""); err != nil {
		return nil, err
	}
	return c, tx.Commit(r.Context())
}

// clubManager 校验当前组织负责人或平台管理员。
func (a *App) clubManager(r *http.Request, u User, id string) (Club, error) {
	c, err := get[Club](r.Context(), a.DB, "clubs", id, false)
	if err != nil {
		return c, err
	}
	if allowed, permissionErr := canManageClub(r.Context(), a.DB, u, c); permissionErr != nil {
		return c, permissionErr
	} else if !allowed {
		return c, fail(403, "无权管理此俱乐部")
	}
	return c, nil
}

// joinClub 幂等提交挂靠申请，已经通过的申请不会降级。
func (a *App) joinClub(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		ApplicationNote      string `json:"application_note"`
		RequirementsAccepted bool   `json:"requirements_accepted"`
	}
	if r.ContentLength != 0 {
		if err = decode(r, &in); err != nil {
			return nil, err
		}
	}
	if !length(in.ApplicationNote, 0, 500) {
		return nil, fail(400, "申请说明最多 500 字")
	}
	preflightClub, checkErr := get[Club](r.Context(), a.DB, "clubs", r.PathValue("id"), false)
	if checkErr != nil {
		return nil, checkErr
	}
	if preflightClub.Status != "active" {
		return nil, fail(400, "俱乐部未开放挂靠")
	}
	if err = a.moderateText(r, u, "俱乐部挂靠申请", in.ApplicationNote); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = lockClubFinance(r.Context(), tx, r.PathValue("id")); err != nil {
		return nil, err
	}
	if err = guardClubDeposit(r.Context(), tx, r.PathValue("id")); err != nil {
		return nil, err
	}
	c, err := get[Club](r.Context(), tx, "clubs", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if c.Status != "active" {
		return nil, fail(400, "俱乐部未开放挂靠")
	}
	if c.OwnerID == u.ID {
		return map[string]bool{"ok": true}, nil
	}
	if (c.DepositCents > 0 || c.Assessment != "" || c.CommissionRateBPS > 0 || c.JoinNote != "") && !in.RequirementsAccepted {
		return nil, fail(400, "请先阅读并确认入会规则；确认规则不表示押金已支付")
	}
	id := c.ID + ":" + u.ID
	m, e := get[ClubMember](r.Context(), tx, "members", id, true)
	if e != nil && !missing(e) {
		return nil, e
	}
	if e == nil && (m.Status == "active" || m.Status == "pending" && m.AppliedDepositCents == c.DepositCents && m.AppliedAssessment == c.Assessment && m.AppliedCommissionRateBPS == c.CommissionRateBPS && m.AppliedJoinNote == c.JoinNote) {
		return map[string]bool{"ok": true}, nil
	}
	m = ClubMember{UserID: u.ID, Nickname: u.Nickname, ClubID: c.ID, Status: "pending", CreatedAt: now(), ApplicationNote: strings.TrimSpace(in.ApplicationNote), RequirementsAccepted: in.RequirementsAccepted, AppliedCommissionRateBPS: c.CommissionRateBPS, AppliedJoinNote: c.JoinNote, AppliedDepositCents: c.DepositCents, AppliedAssessment: c.Assessment}
	if err = put(r.Context(), tx, "members", id, m); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "club.apply", id, "申请不代表认证或押金支付"); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, tx.Commit(r.Context())
}

// leaveClub 撤销当前挂靠，历史订单的组织归属仍保留。
func (a *App) leaveClub(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	c, err := get[Club](r.Context(), a.DB, "clubs", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if c.OwnerID == u.ID {
		return nil, fail(400, "负责人不能退出自己创建的俱乐部")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = lockClubFinance(r.Context(), tx, c.ID); err != nil {
		return nil, err
	}
	id := c.ID + ":" + u.ID
	m, err := get[ClubMember](r.Context(), tx, "members", id, true)
	if missing(err) {
		return map[string]bool{"ok": true}, nil
	}
	if err != nil {
		return nil, err
	}
	if err = revokeClubDeputy(r.Context(), tx, c.ID, u.ID, u.ID); err != nil {
		return nil, err
	}
	m.Status = "left"
	if err = put(r.Context(), tx, "members", id, m); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "club.leave", c.ID, ""); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, tx.Commit(r.Context())
}

// moderateMember 将组织负责人操作限制在本组织申请记录。
func (a *App) moderateMember(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	c, err := a.clubManager(r, u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	var in struct {
		Status           string `json:"status"`
		AssessmentPassed bool   `json:"assessment_passed"`
		ReviewNote       string `json:"review_note"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"active", "rejected", "removed"}, in.Status) || !length(in.ReviewNote, 0, 500) {
		return nil, fail(400, "成员状态无效")
	}
	if err = a.moderateText(r, u, "俱乐部成员审核说明", in.ReviewNote); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = lockClubFinance(r.Context(), tx, c.ID); err != nil {
		return nil, err
	}
	u, err = lockManagementWriteActor(r.Context(), tx, u.ID, r.PathValue("user_id"))
	if err != nil {
		return nil, err
	}
	if in.Status == "active" {
		if err = guardClubDeposit(r.Context(), tx, c.ID); err != nil {
			return nil, err
		}
	}
	id := c.ID + ":" + r.PathValue("user_id")
	c, err = get[Club](r.Context(), tx, "clubs", c.ID, true)
	if err != nil {
		return nil, err
	}
	if allowed, permissionErr := canManageClub(r.Context(), tx, u, c); permissionErr != nil {
		return nil, permissionErr
	} else if !allowed {
		return nil, fail(403, "无权管理此俱乐部")
	}
	m, err := get[ClubMember](r.Context(), tx, "members", id, true)
	if err != nil {
		return nil, err
	}
	if in.Status == "active" {
		if c.Status != "active" || m.Status == "left" {
			return nil, fail(409, "俱乐部需通过审核，成员退出后需重新申请")
		}
		if m.AppliedCommissionRateBPS != c.CommissionRateBPS || m.AppliedJoinNote != c.JoinNote || m.AppliedDepositCents != c.DepositCents || m.AppliedAssessment != c.Assessment || (c.DepositCents > 0 || c.Assessment != "" || c.CommissionRateBPS > 0 || c.JoinNote != "") && !m.RequirementsAccepted {
			return nil, fail(409, "入会规则已经变化，请成员重新确认后申请")
		}
		if c.Assessment != "" && !in.AssessmentPassed {
			return nil, fail(400, "请完成并确认考核后再通过认证")
		}
		wallet, e := accountBalance(r.Context(), tx, memberDepositAccount(c.ID, m.UserID))
		if e != nil {
			return nil, e
		}
		if wallet.DepositRefundingCents > 0 || wallet.DepositCents < c.DepositCents {
			return nil, fail(409, "成员尚未实缴加入规则要求的俱乐部托管押金")
		}
		member, e := get[User](r.Context(), tx, "users", m.UserID, false)
		if e != nil || member.Status != "active" {
			return nil, fail(400, "该成员账号当前不可认证")
		}
	}
	if in.Status != "active" {
		if deputy, e := get[ClubDeputy](r.Context(), tx, "club_deputies", id, false); e == nil && deputy.Status == "active" && !canManageClubAdmins(u, c) {
			return nil, fail(403, "副管理员不能通过移除成员撤销管理员")
		} else if e != nil && !missing(e) {
			return nil, e
		}
		if err = revokeClubDeputy(r.Context(), tx, c.ID, m.UserID, u.ID); err != nil {
			return nil, err
		}
	}
	m.Status = in.Status
	m.AssessmentPassed = in.Status == "active" && in.AssessmentPassed
	m.ReviewNote = strings.TrimSpace(in.ReviewNote)
	if err = put(r.Context(), tx, "members", id, m); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "club.member."+in.Status, id, m.ReviewNote); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, tx.Commit(r.Context())
}

// clubOrders 只展示下单时锁定归属该组织的订单。
func (a *App) clubOrders(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	c, err := a.clubManager(r, u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	items, err := list[Order](r.Context(), a.DB, "orders", "WHERE data->>'club_id'=$1 ORDER BY data->>'created_at' DESC", c.ID)
	return paginate(r, items), err
}
