// Package app 提供安全用户摘要、资料管理和关联记录只读分页，资金与微信身份不接受管理表单覆盖。
package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ManagementPermissions 由服务端为管理详情计算可执行动作，不作为请求入参。
type ManagementPermissions struct {
	CanEdit         bool `json:"can_edit"`
	CanManageAdmins bool `json:"can_manage_admins"`
	CanSendMessage  bool `json:"can_send_message"`
}

// ManagementPage 在数据库范围过滤后分页，客户端不必猜测是否仍有下一页。
type ManagementPage[T any] struct {
	Items   []T  `json:"items"`
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// ManagedUserSummary 不含简介、openid及完整手机号码，后台列表也遵守最少必要字段。
type ManagedUserSummary struct {
	ID                string `json:"id"`
	Nickname          string `json:"nickname"`
	Avatar            string `json:"avatar"`
	PhoneMasked       string `json:"phone_masked"`
	PhoneVerified     bool   `json:"phone_verified"`
	Role              string `json:"role"`
	PlatformAdminRole string `json:"platform_admin_role"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
}

// managementUserSummary 将持久用户投影成列表摘要，历史管理员继续显示根角色。
func managementUserSummary(u User) ManagedUserSummary {
	role := u.PlatformAdminRole
	if isPlatformRoot(u) {
		role = "root"
	}
	return ManagedUserSummary{u.ID, u.Nickname, u.Avatar, u.PhoneMasked, u.PhoneVerified, u.Role, role, u.Status, u.CreatedAt}
}

// managedPage 仅拼接内部白名单SQL，搜索范围在LIMIT前确定并返回准确总数。
func managedPage[T any](ctx context.Context, q querier, table, where string, args []any, r *http.Request) (ManagementPage[T], error) {
	limit, offset := pageLimit(r)
	out := ManagementPage[T]{Items: []T{}, Limit: limit, Offset: offset}
	if err := q.QueryRow(ctx, "SELECT count(*) FROM "+validTable(table)+" WHERE "+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	values := append(append([]any{}, args...), limit, offset)
	items, err := list[T](ctx, q, table, "WHERE "+where+fmt.Sprintf(" ORDER BY data->>'created_at' DESC,id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2), values...)
	out.Items = items
	out.HasMore = offset+len(items) < out.Total
	return out, err
}

// registerAdminCentersRoutes 将用户及成员管理接口独立挂载，所有处理器重新检查服务端权限。
func (a *App) registerAdminCentersRoutes(mux *http.ServeMux) {
	routes := map[string]func(http.ResponseWriter, *http.Request) (any, error){
		"GET /api/admin/users/{id}": a.adminUserDetail, "PATCH /api/admin/users/{id}/profile": a.adminUserProfile, "PATCH /api/admin/users/{id}/deputy": a.setPlatformDeputy, "POST /api/admin/users/{id}/messages": a.adminUserMessage,
		"GET /api/admin/users/{id}/wallet": a.adminUserWallet, "GET /api/admin/users/{id}/wallet/entries": a.adminUserEntries, "GET /api/admin/users/{id}/orders": a.adminUserOrders, "GET /api/admin/users/{id}/moderation": a.adminUserModeration,
		"GET /api/clubs/{id}/members": a.managedMembers, "GET /api/clubs/{id}/members/{user_id}": a.managedMemberDetail, "PATCH /api/clubs/{id}/deputies/{user_id}": a.setClubDeputy, "POST /api/clubs/{id}/members/{user_id}/messages": a.managedMemberMessage,
		"GET /api/clubs/{id}/members/{user_id}/orders": a.managedMemberOrders, "GET /api/clubs/{id}/members/{user_id}/finance/entries": a.managedMemberEntries, "GET /api/clubs/{id}/members/{user_id}/moderation": a.managedMemberModeration,
	}
	for route, handler := range routes {
		mux.HandleFunc(route, a.wrap(handler))
	}
}

// adminTarget 校验管理者及目标用户存在，不读取私密身份映射。
func (a *App) adminTarget(r *http.Request) (User, User, error) {
	actor, err := a.admin(r)
	if err != nil {
		return actor, User{}, err
	}
	target, err := get[User](r.Context(), a.DB, "users", r.PathValue("id"), false)
	return actor, target, err
}

// adminUserDetail 返回编辑所需公开资料及真实权限，副管理员不能编辑管理员身份相关资料。
func (a *App) adminUserDetail(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, u, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	return map[string]any{"user": u, "permissions": ManagementPermissions{actor.Role == "admin" && (isPlatformRoot(actor) || u.Role != "admin"), isPlatformRoot(actor) && !isPlatformRoot(u), true}}, nil
}

// adminUserProfile 只修改展示字段，未知身份、金额和手机号字段由严格JSON解码拒绝。
func (a *App) adminUserProfile(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, target, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	if !isPlatformRoot(actor) && target.Role == "admin" {
		return nil, fail(403, "副管理员不能修改管理员资料")
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
	for _, image := range []struct {
		value           *string
		before, purpose string
	}{{in.Avatar, target.Avatar, "avatar"}, {in.ProfileCover, target.ProfileCover, "profile_cover"}} {
		if image.value == nil || *image.value == image.before {
			continue
		}
		owner := target.ID
		if match := publicImagePath.FindStringSubmatch(*image.value); match != nil {
			v, e := get[PublicImage](r.Context(), a.DB, "evidence", match[1], false)
			if e == nil && v.OwnerID == actor.ID {
				owner = actor.ID
			}
		}
		if err = a.moderateOwnedImage(r, actor, *image.value, image.purpose, owner, ""); err != nil {
			return nil, err
		}
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	actor, err = lockManagementWriteActor(ctx, tx, actor.ID, target.ID)
	if err != nil {
		return nil, err
	}
	if actor.Role != "admin" {
		return nil, fail(403, "平台管理权限已失效")
	}
	target, err = get[User](ctx, tx, "users", target.ID, true)
	if err != nil {
		return nil, err
	}
	if !isPlatformRoot(actor) && target.Role == "admin" {
		return nil, fail(403, "副管理员不能修改管理员资料")
	}
	if in.Nickname != nil {
		target.Nickname = strings.TrimSpace(*in.Nickname)
	}
	if in.Bio != nil {
		target.Bio = strings.TrimSpace(*in.Bio)
	}
	if in.Avatar != nil {
		target.Avatar = *in.Avatar
	}
	if in.ProfileCover != nil {
		target.ProfileCover = *in.ProfileCover
	}
	if err = put(ctx, tx, "users", target.ID, target); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, actor.ID, "user.profile.edit", target.ID, "修改公开资料，未更改身份或资金"); err != nil {
		return nil, err
	}
	return target, tx.Commit(ctx)
}

// sendManagedUserMessage 向已确定的单个账号投递站内通知，正文仍经过后置审核，微信订阅范围不扩张。
func (a *App) sendManagedUserMessage(r *http.Request, actor User, target, club string) (any, error) {
	if err := a.throttle(r.Context(), "managed-notice", actor.ID, 20); err != nil {
		return nil, err
	}
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if !length(in.Title, 1, 60) || !length(in.Body, 1, 5000) {
		return nil, fail(400, "消息标题1至60字、正文1至5000字")
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockClubFinance(ctx, tx, club); err != nil {
		return nil, err
	}
	actor, err = lockManagementWriteActor(ctx, tx, actor.ID, target)
	if err != nil {
		return nil, err
	}
	if club == "" && actor.Role != "admin" {
		return nil, fail(403, "平台消息管理权限已失效")
	}
	u, err := get[User](ctx, tx, "users", target, false)
	if err != nil || u.Status != "active" {
		return nil, fail(404, "接收用户不存在或已停用")
	}
	if club != "" {
		c, e := get[Club](ctx, tx, "clubs", club, false)
		if e != nil {
			return nil, e
		}
		allowed, e := canManageClub(ctx, tx, actor, c)
		if e != nil {
			return nil, e
		}
		if !allowed {
			return nil, fail(403, "无权向此组织成员发送消息")
		}
		if _, e = get[ClubMember](ctx, tx, "members", club+":"+target, false); e != nil {
			return nil, fail(404, "成员不属于此俱乐部")
		}
	}
	v := InboxMessage{ID: newID(), UserID: target, Title: in.Title, Body: in.Body, CreatedAt: now()}
	if err = put(ctx, tx, "inbox", v.ID, v); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, actor.ID, "user.message", target, "发送定向站内通知，俱乐部="+club); err != nil {
		return nil, err
	}
	return map[string]int{"recipient_count": 1}, tx.Commit(ctx)
}

// adminUserMessage 把接收人固定为路径目标，不能通过请求体改成群发或其它用户。
func (a *App) adminUserMessage(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, u, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	return a.sendManagedUserMessage(r, actor, u.ID, "")
}

// adminUserWallet 只读该用户钱包，不自动触发结算或改变任何账户。
func (a *App) adminUserWallet(w http.ResponseWriter, r *http.Request) (any, error) {
	_, u, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	v, err := accountBalance(r.Context(), a.DB, u.ID)
	if err != nil {
		return nil, err
	}
	return a.walletSummaryValue(r.Context(), v, "")
}

// adminUserEntries 先绑定目标账号后分页返回不可变账单。
func (a *App) adminUserEntries(w http.ResponseWriter, r *http.Request) (any, error) {
	_, u, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	return managedPage[WalletEntry](r.Context(), a.DB, "wallet_entries", "data->>'user_id'=$1", []any{u.ID}, r)
}

// managedOrderStatus 仅允许订单实际状态筛选，参数绑定保持范围条件不可被覆盖。
func managedOrderStatus(r *http.Request, where string, args []any) (string, []any, error) {
	status := r.URL.Query().Get("status")
	if status == "" || status == "all" {
		return where, args, nil
	}
	if !contains([]string{"pending_payment", "open", "accepted", "in_progress", "delivered", "completed", "cancelled", "disputed", "refunded"}, status) {
		return "", nil, fail(400, "订单状态无效")
	}
	args = append(args, status)
	return where + fmt.Sprintf(" AND data->>'status'=$%d", len(args)), args, nil
}

// adminUserOrders 将买卖身份限制在指定用户，客户端role不能扩大到全平台订单。
func (a *App) adminUserOrders(w http.ResponseWriter, r *http.Request) (any, error) {
	_, u, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	where := "(data->>'buyer_id'=$1 OR data->>'provider_id'=$1)"
	switch r.URL.Query().Get("role") {
	case "", "all":
	case "buyer":
		where = "data->>'buyer_id'=$1"
	case "provider":
		where = "data->>'provider_id'=$1"
	default:
		return nil, fail(400, "订单身份筛选无效")
	}
	where, args, err := managedOrderStatus(r, where, []any{u.ID})
	if err != nil {
		return nil, err
	}
	page, e := managedPage[Order](r.Context(), a.DB, "orders", where, args, r)
	page.Items = publicValue(page.Items).([]Order)
	return page, e
}

// managementModerationRecord 不返回模型密钥、私密身份及交易原证据预览。
func managementModerationRecord(v ModerationTask) map[string]any {
	return map[string]any{"id": v.ID, "target_type": v.TargetType, "target_id": v.TargetID, "field_label": v.FieldLabel, "kind": v.Kind, "status": v.Status, "reason": v.Reason, "decision_source": v.DecisionSource, "attempts": v.Attempts, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt}
}

// managementModerationPage 在既定归属范围内筛选审核状态并分页，仍保持只读。
func (a *App) managementModerationPage(r *http.Request, where string, args []any) (any, error) {
	if status := r.URL.Query().Get("status"); status != "" && status != "all" {
		if !contains(reviewStatuses, status) {
			return nil, fail(400, "审核状态无效")
		}
		args = append(args, status)
		where += fmt.Sprintf(" AND data->>'status'=$%d", len(args))
	}
	page, err := managedPage[ModerationTask](r.Context(), a.DB, "moderation_tasks", where, args, r)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for _, v := range page.Items {
		items = append(items, managementModerationRecord(v))
	}
	return ManagementPage[map[string]any]{items, page.Total, page.Limit, page.Offset, page.HasMore}, nil
}

// adminUserModeration 返回目标用户所属或操作的图文审核记录，AI调用与人工结果均如实标识。
func (a *App) adminUserModeration(w http.ResponseWriter, r *http.Request) (any, error) {
	_, u, err := a.adminTarget(r)
	if err != nil {
		return nil, err
	}
	return a.managementModerationPage(r, "(data->>'owner_id'=$1 OR data->>'actor_id'=$1 OR (data->>'target_type'='users' AND data->>'target_id'=$1))", []any{u.ID})
}
