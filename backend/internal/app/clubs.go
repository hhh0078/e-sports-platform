// Package app 实现多俱乐部隔离、挂靠审核、组织认证及历史订单权限。
package app

import (
	"context"
	"net/http"
	"strings"
)

// decorateClub 计算当前有效成员数量与负责人昵称。
func (a *App) decorateClub(ctx context.Context, c Club) Club {
	if u, e := get[User](ctx, a.DB, "users", c.OwnerID, false); e == nil {
		c.OwnerName = u.Nickname
	}
	var n int
	if a.DB.QueryRow(ctx, "SELECT count(*) FROM members WHERE data->>'club_id'=$1 AND data->>'status'='active'", c.ID).Scan(&n) == nil {
		c.MemberCount = n
	}
	return c
}

// clubs 返回公开俱乐部或当前用户创建及申请挂靠的俱乐部。
func (a *App) clubs(w http.ResponseWriter, r *http.Request) (any, error) {
	mine := r.URL.Query().Get("mine") == "1"
	var u User
	var err error
	if mine {
		u, err = a.actor(r)
		if err != nil {
			return nil, err
		}
	}
	items, err := list[Club](r.Context(), a.DB, "clubs", "ORDER BY data->>'created_at' DESC")
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
	manager := u.Role == "admin" || u.ID == c.OwnerID
	if c.Status != "active" && !manager {
		return nil, fail(404, "俱乐部尚未通过审核")
	}
	members, err := list[ClubMember](r.Context(), a.DB, "members", "WHERE data->>'club_id'=$1 ORDER BY data->>'created_at'", c.ID)
	if err != nil {
		return nil, err
	}
	out := []ClubMember{}
	var my *ClubMember
	for _, m := range members {
		if member, e := get[User](r.Context(), a.DB, "users", m.UserID, false); e == nil {
			m.Nickname = member.Nickname
		}
		if m.UserID == u.ID {
			copy := m
			my = &copy
		}
		if manager || m.Status == "active" || m.UserID == u.ID {
			out = append(out, m)
		}
	}
	return map[string]any{"club": a.decorateClub(r.Context(), c), "members": out, "my_membership": my}, nil
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

// saveClub 创建或编辑组织，修改资料将重新审核并暂时撤销认证。
func (a *App) saveClub(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		GameIDs     []string `json:"game_ids"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	c := Club{ID: newID(), OwnerID: u.ID, OwnerName: u.Nickname, CreatedAt: now()}
	if id := r.PathValue("id"); id != "" {
		c, err = get[Club](r.Context(), tx, "clubs", id, true)
		if err != nil {
			return nil, err
		}
		if c.OwnerID != u.ID && u.Role != "admin" {
			return nil, fail(403, "只能管理自己的俱乐部")
		}
	}
	c.Name = strings.TrimSpace(in.Name)
	c.Description = strings.TrimSpace(in.Description)
	c.GameIDs = in.GameIDs
	c.Status = "pending"
	c.Verified = false
	if err = validateClub(r.Context(), tx, c); err != nil {
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
	if c.OwnerID != u.ID && u.Role != "admin" {
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
	if !u.ProviderEnabled {
		return nil, fail(400, "请先开启打手身份")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
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
	id := c.ID + ":" + u.ID
	m, e := get[ClubMember](r.Context(), tx, "members", id, true)
	if e != nil && !missing(e) {
		return nil, e
	}
	if e == nil && contains([]string{"pending", "active"}, m.Status) {
		return map[string]bool{"ok": true}, nil
	}
	m = ClubMember{UserID: u.ID, Nickname: u.Nickname, ClubID: c.ID, Status: "pending", CreatedAt: now()}
	if err = put(r.Context(), tx, "members", id, m); err != nil {
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
	id := c.ID + ":" + u.ID
	m, err := get[ClubMember](r.Context(), tx, "members", id, true)
	if missing(err) {
		return map[string]bool{"ok": true}, nil
	}
	if err != nil {
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
		Status string `json:"status"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"active", "rejected"}, in.Status) {
		return nil, fail(400, "成员状态无效")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	id := c.ID + ":" + r.PathValue("user_id")
	m, err := get[ClubMember](r.Context(), tx, "members", id, true)
	if err != nil {
		return nil, err
	}
	m.Status = in.Status
	if err = put(r.Context(), tx, "members", id, m); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "club.member."+in.Status, id, ""); err != nil {
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
