// Package app 提供四方订单沟通与完成后评价，消息证据不可编辑删除。
package app

import (
	"net/http"
	"strings"
)

// senderRole 从服务端身份及订单关系确定聊天角色。
func (a *App) senderRole(r *http.Request, u User, o Order) string {
	if u.Role == "admin" {
		return "admin"
	}
	if u.ID == o.BuyerID {
		return "buyer"
	}
	if u.ID == o.ProviderID {
		return "provider"
	}
	return "club_admin"
}

// messages 读取当前订单不可变的沟通证据。
func (a *App) messages(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	o, err := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	where := "WHERE data->>'order_id'=$1"
	args := []any{o.ID, limit, offset}
	beforeID, afterID := r.URL.Query().Get("before_id"), r.URL.Query().Get("after_id")
	if beforeID != "" && afterID != "" {
		return nil, fail(400, "历史与增量游标不能同时使用")
	}
	cursor, op := beforeID, "<"
	if afterID != "" {
		cursor = afterID
		op = ">"
	}
	if cursor != "" {
		anchor, e := get[Message](r.Context(), a.DB, "messages", cursor, false)
		if e != nil || anchor.OrderID != o.ID {
			return nil, fail(400, "消息游标不存在或不属于此订单")
		}
		where += " AND (data->>'created_at',id)" + op + "($4,$5)"
		args = append(args, anchor.CreatedAt, anchor.ID)
	}
	ordering := " ORDER BY data->>'created_at' DESC,id DESC"
	if afterID != "" {
		ordering = " ORDER BY data->>'created_at' ASC,id ASC"
	}
	items, err := list[Message](r.Context(), a.DB, "messages", where+ordering+" LIMIT $2 OFFSET $3", args...)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ImageURL != "" {
			items[i].ImageURL = a.signEvidenceURL(items[i].ImageURL)
		}
	}
	if afterID == "" {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	return items, nil
}

// message 写入文字或本订单上传的图片，客户端不能冒充系统或其他人。
func (a *App) message(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	o, err := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if err = requireLiveOrder(o); err != nil {
		return nil, err
	}
	if err = a.throttle(r.Context(), "message", u.ID, 60); err != nil {
		return nil, err
	}
	var in struct {
		Content  string `json:"content"`
		ImageURL string `json:"image_url"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	in.Content = strings.TrimSpace(in.Content)
	if !length(in.Content, 0, 2000) || (in.Content == "" && in.ImageURL == "") {
		return nil, fail(400, "请填写消息或选择图片，文字最多 2000 字")
	}
	m := Message{ID: newID(), OrderID: o.ID, SenderID: u.ID, SenderName: u.Nickname, SenderRole: a.senderRole(r, u, o), Content: in.Content, Kind: "text", CreatedAt: now()}
	if in.ImageURL != "" {
		raw, err := a.validateEvidence(r, o.ID, in.ImageURL)
		if err != nil {
			return nil, err
		}
		m.ImageURL = raw
		m.Kind = "image"
	}
	if err = put(r.Context(), a.DB, "messages", m.ID, m); err != nil {
		return nil, err
	}
	if m.ImageURL != "" {
		m.ImageURL = a.signEvidenceURL(m.ImageURL)
	}
	return m, nil
}

// review 只允许已完成订单买家评价一次，唯一约束作为并发保护。
func (a *App) review(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Rating  int    `json:"rating"`
		Content string `json:"content"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Rating < 1 || in.Rating > 5 || !length(in.Content, 2, 500) {
		return nil, fail(400, "评分范围 1 至 5，评价 2 至 500 字")
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
	if o.BuyerID != u.ID {
		return nil, fail(403, "只有买家可以评价")
	}
	if o.Status != "completed" || contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) {
		return nil, fail(409, "订单尚未完成或处于售后处理中")
	}
	var exists bool
	if err = tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM reviews WHERE data->>'order_id'=$1)", o.ID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, fail(409, "该订单已评价")
	}
	v := Review{ID: newID(), OrderID: o.ID, Rating: in.Rating, Content: strings.TrimSpace(in.Content), BuyerName: u.Nickname, CreatedAt: now()}
	if err = put(r.Context(), tx, "reviews", v.ID, v); err != nil {
		return nil, err
	}
	if err = refreshServiceStats(r.Context(), tx, o.ServiceID); err != nil {
		return nil, err
	}
	return v, tx.Commit(r.Context())
}

// reviews 返回有权访问订单的评价列表。
func (a *App) reviews(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	o, err := a.authorizedOrder(r, u, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	return list[Review](r.Context(), a.DB, "reviews", "WHERE data->>'order_id'=$1", o.ID)
}
