// Package app 持久化平台及俱乐部副管理员权限，授权关系与组织成员绑定且不信任客户端角色。
package app

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

// ClubDeputy 保存一名成员在单个俱乐部的授权历史，撤销后不删除操作者记录。
type ClubDeputy struct {
	ClubID     string `json:"club_id"`
	UserID     string `json:"user_id"`
	Status     string `json:"status"`
	AssignedBy string `json:"assigned_by"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// isPlatformRoot 保留历史管理员根权限，仅显式deputy记录属于平台副管理员。
func isPlatformRoot(u User) bool {
	return u.Role == "admin" && (u.PlatformAdminRole == "" || u.PlatformAdminRole == "root")
}

// lockManagementWriteActor 按用户ID统一锁顺序重读操作者，撤权或冻结先提交时旧请求不能继续写入。
// targets 仅由处理器传入真实业务目标；操作者行锁保持到原业务事务结束，禁止检查后并发失权。
func lockManagementWriteActor(ctx context.Context, q querier, actorID string, targets ...string) (User, error) {
	ids := append([]string{actorID}, targets...)
	users, err := list[User](ctx, q, "users", "WHERE id=ANY($1) ORDER BY id FOR UPDATE", ids)
	if err != nil {
		return User{}, err
	}
	for _, user := range users {
		if user.ID == actorID && user.Status == "active" {
			return user, nil
		}
	}
	return User{}, fail(403, "管理操作者身份已失效")
}

// canManageClubAdmins 将组织授权限定给负责人和平台根管理员，平台副管理员也不得授撤组织角色。
func canManageClubAdmins(u User, c Club) bool {
	return u.Status == "active" && (isPlatformRoot(u) || u.Role != "admin" && u.ID == c.OwnerID)
}

// managedClubWhere 返回内部SQL条件；先限定本人及有效成员授权，再由调用者搜索分页。
func managedClubWhere(alias, actorParam string) string {
	if !regexp.MustCompile(`^[a-z_]+$`).MatchString(alias) || !regexp.MustCompile(`^\$[1-9][0-9]*$`).MatchString(actorParam) {
		panic("非法管理范围SQL标识")
	}
	return "(EXISTS(SELECT 1 FROM users managing_actor WHERE managing_actor.id=" + actorParam + " AND managing_actor.data->>'status'='active') AND (" + alias + ".data->>'owner_id'=" + actorParam + " OR EXISTS(SELECT 1 FROM club_deputies d JOIN members m ON m.id=" + alias + ".id||':'||" + actorParam + " AND m.data->>'club_id'=" + alias + ".id AND m.data->>'user_id'=" + actorParam + " AND m.data->>'status'='active' WHERE d.id=" + alias + ".id||':'||" + actorParam + " AND d.data->>'club_id'=" + alias + ".id AND d.data->>'user_id'=" + actorParam + " AND d.data->>'status'='active')))"
}

// clubManagementRole 从数据库实时判断负责人或有效副管理员，不将平台管理员虚构为本人组织。
func clubManagementRole(ctx context.Context, q querier, u User, c Club) (string, error) {
	if u.ID == "" || u.Status != "active" {
		return "", nil
	}
	if c.OwnerID == u.ID {
		return "owner", nil
	}
	var allowed bool
	err := q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM clubs WHERE id=$1 AND "+managedClubWhere("clubs", "$2")+")", c.ID, u.ID).Scan(&allowed)
	if allowed {
		return "deputy", err
	}
	return "", err
}

// canManageClub 供组织业务共同鉴权；平台管理范围与本人owner/deputy关系分别保留。
func canManageClub(ctx context.Context, q querier, u User, c Club) (bool, error) {
	if u.Role == "admin" && u.Status == "active" {
		return true, nil
	}
	role, err := clubManagementRole(ctx, q, u, c)
	return role != "", err
}

// clubManagementRecipients 为既有俱乐部申请和争议提醒选择当前有效负责人及副管理员，不新增通知场景。
func clubManagementRecipients(ctx context.Context, q querier, clubID string) ([]string, error) {
	out := []string{}
	if clubID == "" || strings.HasPrefix(clubID, "demo_") {
		return out, nil
	}
	c, err := get[Club](ctx, q, "clubs", clubID, false)
	if missing(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	users, err := list[User](ctx, q, "users", `WHERE data->>'status'='active' AND (id=$1 OR EXISTS(SELECT 1 FROM club_deputies d JOIN members m ON m.id=$2||':'||users.id AND m.data->>'status'='active' AND m.data->>'club_id'=$2 AND m.data->>'user_id'=users.id WHERE d.id=$2||':'||users.id AND d.data->>'club_id'=$2 AND d.data->>'user_id'=users.id AND d.data->>'status'='active'))`, c.OwnerID, c.ID)
	for _, u := range users {
		out = append(out, u.ID)
	}
	return out, err
}

// isClubManagementRecipient 在实际发送前重新核对组织关系，撤权后不消耗一次性授权。
func isClubManagementRecipient(ctx context.Context, q querier, userID string, c Club) (bool, error) {
	u, err := get[User](ctx, q, "users", userID, false)
	if missing(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	role, err := clubManagementRole(ctx, q, u, c)
	return role != "", err
}

// migrateManagementPermissions 为授权关系建立中文注释索引，历史根管理员不自动降权。
func migrateManagementPermissions(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `CREATE INDEX IF NOT EXISTS club_deputies_scope_idx ON club_deputies((data->>'club_id'),(data->>'user_id'),(data->>'status'));
	COMMENT ON INDEX club_deputies_scope_idx IS '按俱乐部、用户与有效状态核验副管理员权限';`)
	return err
}

// revokeClubDeputy 在成员离会或被移除的同一组织事务内撤权，不保留失效的管理入口。
func revokeClubDeputy(ctx context.Context, q querier, club, user, actor string) error {
	id := club + ":" + user
	v, err := get[ClubDeputy](ctx, q, "club_deputies", id, true)
	if missing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if v.Status != "active" {
		return nil
	}
	v.Status = "revoked"
	v.AssignedBy = actor
	v.UpdatedAt = now()
	if err = put(ctx, q, "club_deputies", id, v); err != nil {
		return err
	}
	return audit(ctx, q, actor, "club.deputy.revoke", id, "成员关系失效同步撤销")
}

// setPlatformDeputy 只有当前根管理员能授撤副管理员，目标根管理员永远不能经此降级。
func (a *App) setPlatformDeputy(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	if !isPlatformRoot(actor) {
		return nil, fail(403, "仅平台根管理员可授撤副管理员")
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Enabled == nil {
		return nil, fail(400, "请明确是否授予副管理员")
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('platform-admin-grants',0))"); err != nil {
		return nil, err
	}
	actor, err = lockManagementWriteActor(ctx, tx, actor.ID, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if actor.Status != "active" || !isPlatformRoot(actor) {
		return nil, fail(403, "根管理员权限已失效")
	}
	u, err := get[User](ctx, tx, "users", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if isPlatformRoot(u) {
		return nil, fail(403, "根管理员不能通过副管理员接口变更")
	}
	if *in.Enabled {
		if u.Status != "active" {
			return nil, fail(409, "停用用户不能授予管理员")
		}
		u.Role = "admin"
		u.PlatformAdminRole = "deputy"
	} else if u.Role == "admin" && u.PlatformAdminRole == "deputy" {
		u.Role = "user"
		u.PlatformAdminRole = ""
	}
	if err = put(ctx, tx, "users", u.ID, u); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, actor.ID, "user.deputy", u.ID, "设置平台副管理员"); err != nil {
		return nil, err
	}
	return u, tx.Commit(ctx)
}

// setClubDeputy 只允许负责人或平台根管理员给本组织已认证用户授权，不接收任何owner或全局角色字段。
func (a *App) setClubDeputy(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Enabled == nil {
		return nil, fail(400, "请明确授权状态")
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockClubFinance(ctx, tx, r.PathValue("id")); err != nil {
		return nil, err
	}
	c, err := get[Club](ctx, tx, "clubs", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	u, err = lockManagementWriteActor(ctx, tx, u.ID, r.PathValue("user_id"))
	if err != nil {
		return nil, err
	}
	if !canManageClubAdmins(u, c) {
		return nil, fail(403, "仅负责人或平台根管理员可授撤俱乐部副管理员")
	}
	user := r.PathValue("user_id")
	if user == c.OwnerID {
		return nil, fail(403, "负责人身份不能变更")
	}
	member, err := get[ClubMember](ctx, tx, "members", c.ID+":"+user, true)
	if err != nil {
		return nil, fail(404, "该组织成员不存在")
	}
	if *in.Enabled {
		target, e := get[User](ctx, tx, "users", user, false)
		if e != nil || target.Status != "active" || member.Status != "active" {
			return nil, fail(409, "只能授权本俱乐部有效认证成员")
		}
	}
	id := c.ID + ":" + user
	record, e := get[ClubDeputy](ctx, tx, "club_deputies", id, true)
	if e != nil && !missing(e) {
		return nil, e
	}
	if record.CreatedAt == "" {
		record = ClubDeputy{ClubID: c.ID, UserID: user, CreatedAt: now()}
	}
	record.Status = "revoked"
	if *in.Enabled {
		record.Status = "active"
	}
	record.AssignedBy = u.ID
	record.UpdatedAt = now()
	if err = put(ctx, tx, "club_deputies", id, record); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, u.ID, "club.deputy", id, "设置俱乐部副管理员"); err != nil {
		return nil, err
	}
	return record, tx.Commit(ctx)
}
