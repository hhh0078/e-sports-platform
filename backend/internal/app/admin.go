// Package app 提供小程序内管理后台，全部写入操作带服务端权限与审计。
package app

import (
	"net/http"
	"strings"
)

// overview 汇总真实业务经营与待办数量，排除演示记录，金额只计最终完成订单。
func (a *App) overview(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	out := map[string]int64{}
	const liveData = "left(id,5)<>'demo_' AND COALESCE(data->>'is_demo','false')<>'true'"
	for _, table := range []string{"users", "services", "orders", "clubs"} {
		var n int64
		if err := a.DB.QueryRow(r.Context(), "SELECT count(*) FROM "+table+" WHERE "+liveData).Scan(&n); err != nil {
			return nil, err
		}
		out[table] = n
	}
	queries := map[string]string{
		"revenue_cents":    "SELECT COALESCE(sum((data->>'amount_cents')::bigint),0) FROM orders WHERE " + liveData + " AND data->>'status'='completed' AND data->>'refund_status' NOT IN ('requested','escalated','processing')",
		"pending_services": "SELECT count(*) FROM services WHERE " + liveData + " AND data->>'status'='pending'",
		"pending_clubs":    "SELECT count(*) FROM clubs WHERE " + liveData + " AND data->>'status'='pending'",
		"disputed_orders":  "SELECT count(*) FROM orders WHERE " + liveData + " AND (data->>'refund_status'='escalated' OR data->>'status'='disputed')",
	}
	for k, q := range queries {
		var n int64
		if err := a.DB.QueryRow(r.Context(), q).Scan(&n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, nil
}

// adminList 返回管理页数据，资源名限定为白名单。
func (a *App) adminList(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	res := r.PathValue("resource")
	if res == "settings" {
		return get[Settings](r.Context(), a.DB, "settings", "platform", false)
	}
	where, args, err := adminQuery(r, res)
	if err != nil {
		return nil, err
	}
	switch res {
	case "settings":
		return get[Settings](r.Context(), a.DB, "settings", "platform", false)
	case "users":
		return list[User](r.Context(), a.DB, "users", where, args...)
	case "services":
		items, e := list[Service](r.Context(), a.DB, "services", where, args...)
		for i := range items {
			items[i] = a.decorateService(r.Context(), items[i])
		}
		return items, e
	case "clubs":
		items, e := list[Club](r.Context(), a.DB, "clubs", where, args...)
		for i := range items {
			items[i] = a.decorateClub(r.Context(), items[i])
		}
		return items, e
	case "orders":
		return list[Order](r.Context(), a.DB, "orders", where, args...)
	case "audit":
		return list[Audit](r.Context(), a.DB, "audit", where, args...)
	case "games":
		return list[Game](r.Context(), a.DB, "games", where, args...)
	}
	return nil, fail(404, "管理资源不存在")
}

// moderateUser 冻结账号立刻阻止既有会话继续访问。
func (a *App) moderateUser(w http.ResponseWriter, r *http.Request) (any, error) {
	admin, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Status string `json:"status"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"active", "frozen"}, in.Status) {
		return nil, fail(400, "用户状态无效")
	}
	if r.PathValue("id") == admin.ID && in.Status == "frozen" {
		return nil, fail(400, "不能停用当前管理员")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	u, err := get[User](r.Context(), tx, "users", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	u.Status = in.Status
	if err = put(r.Context(), tx, "users", u.ID, u); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, admin.ID, "user."+in.Status, u.ID, ""); err != nil {
		return nil, err
	}
	return u, tx.Commit(r.Context())
}

// moderateService 审核服务，不允许客户端覆盖商品或卖家身份。
func (a *App) moderateService(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Status string `json:"status"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"active", "rejected", "paused"}, in.Status) {
		return nil, fail(400, "服务状态无效")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	s, err := get[Service](r.Context(), tx, "services", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if in.Status == "active" {
		g, e := get[Game](r.Context(), tx, "games", s.GameID, false)
		p, pe := get[User](r.Context(), tx, "users", s.ProviderID, false)
		if e != nil || !g.Enabled || pe != nil || p.Status != "active" || !p.ProviderEnabled {
			return nil, fail(409, "游戏或服务提供者当前不可用")
		}
		if err = validateService(&s, g); err != nil {
			return nil, err
		}
	}
	s.Status = in.Status
	if err = put(r.Context(), tx, "services", s.ID, s); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "service."+in.Status, s.ID, ""); err != nil {
		return nil, err
	}
	return s, tx.Commit(r.Context())
}

// moderateClub 审核组织认证，仅审核通过状态允许认证。
func (a *App) moderateClub(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Status   string `json:"status"`
		Verified bool   `json:"verified"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"active", "rejected", "pending"}, in.Status) || in.Verified && in.Status != "active" {
		return nil, fail(400, "仅审核通过的俱乐部可以认证")
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
	c.Status = in.Status
	c.Verified = in.Verified
	if err = put(r.Context(), tx, "clubs", c.ID, c); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "club."+in.Status, c.ID, ""); err != nil {
		return nil, err
	}
	return c, tx.Commit(r.Context())
}

// saveGame 保存管理员配置的游戏风格与封面，更新时保留既有游戏编号。
func (a *App) saveGame(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var g Game
	if err = decode(r, &g); err != nil {
		return nil, err
	}
	if id := r.PathValue("id"); id != "" {
		if g.ID != "" && g.ID != id {
			return nil, fail(400, "游戏编号不可修改")
		}
		g.ID = id
	}
	if err = validateGame(&g); err != nil {
		return nil, err
	}
	for _, cover := range append(append([]string{}, g.Covers...), g.CoverURL, g.IconURL) {
		if strings.HasPrefix(cover, "/api/media/") {
			v, e := get[Evidence](r.Context(), a.DB, "evidence", strings.TrimPrefix(cover, "/api/media/"), false)
			if e != nil || !v.Public {
				return nil, fail(400, "封面不存在或未通过管理员上传")
			}
		}
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	old, e := get[Game](r.Context(), tx, "games", g.ID, true)
	if r.Method == "POST" && !missing(e) {
		if e != nil {
			return nil, e
		}
		return nil, fail(409, "游戏编号已存在")
	}
	if r.Method == "PATCH" && e != nil {
		return nil, e
	}
	_ = old
	if err = put(r.Context(), tx, "games", g.ID, g); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "game.save", g.ID, ""); err != nil {
		return nil, err
	}
	return g, tx.Commit(r.Context())
}

// saveSettings 校验平台运营文字并写入审计。
func (a *App) saveSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in Settings
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !length(in.SiteName, 2, 24) || !length(in.Announcement, 0, 300) {
		return nil, fail(400, "平台名称 2 至 24 字，公告最多 300 字")
	}
	in.SiteName = strings.TrimSpace(in.SiteName)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = put(r.Context(), tx, "settings", "platform", in); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "settings.save", "platform", ""); err != nil {
		return nil, err
	}
	return in, tx.Commit(r.Context())
}

// resolve 平台可裁决退款或恢复履约，不可把未交付订单强制结算。
func (a *App) resolve(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Resolution string `json:"resolution"`
		Note       string `json:"note"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"refund", "resume", "complete"}, in.Resolution) || !length(in.Note, 2, 2000) {
		return nil, fail(400, "请选择有效裁决并填写说明")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	o, err := get[Order](r.Context(), tx, "orders", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if err = requireLiveOrder(o); err != nil {
		return nil, err
	}
	if in.Resolution == "refund" && o.Status == "refunded" && o.RefundStatus == "approved" {
		return o, nil
	}
	if o.RefundStatus != "escalated" && o.Status != "disputed" {
		return nil, fail(409, "该订单没有待平台处理的争议")
	}
	remoteRefund := in.Resolution == "refund" && !a.Config.DevMode
	switch in.Resolution {
	case "refund":
		if remoteRefund {
			before := o
			if err = a.prepareRemoteRefund(&o, before); err != nil {
				return nil, err
			}
		} else {
			o.Status = "refunded"
			o.PaymentStatus = "refunded"
			o.RefundStatus = "approved"
		}
	case "resume":
		o.RefundStatus = "rejected"
	case "complete":
		if !contains([]string{"delivered", "completed"}, o.Status) {
			return nil, fail(409, "尚未交付的订单不能结算")
		}
		o.Status = "completed"
		if o.CompletedAt == "" {
			o.CompletedAt = now()
		}
		o.RefundStatus = "rejected"
	}
	o.RefundReply = in.Note
	o.UpdatedAt = now()
	if err = put(r.Context(), tx, "orders", o.ID, o); err != nil {
		return nil, err
	}
	resolutionName := map[string]string{"refund": "支持买方退款", "resume": "恢复履约", "complete": "确认服务完成"}[in.Resolution]
	if err = systemMessage(r.Context(), tx, o, "平台裁决（"+resolutionName+"）："+in.Note); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "order.resolve."+in.Resolution, o.ID, in.Note); err != nil {
		return nil, err
	}
	if err = refreshServiceStats(r.Context(), tx, o.ServiceID); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	if remoteRefund {
		return a.submitRemoteRefund(r.Context(), o, in.Note)
	}
	return o, nil
}
